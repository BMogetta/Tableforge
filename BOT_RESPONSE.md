Buen documento. Lo leí completo y puedo responder las open questions directamente desde el código que ya conozco, y darte una estimación honesta.

---

**Respuestas a las open questions:**

- **`PlayerID`**: `uuid.UUID` en el store, `engine.PlayerID` (que es `string`) en el engine. El bot necesita ambos.
- **`Player` interface vs struct**: struct concreto (`store.Player`). No hay interface. Esto es el cambio más invasivo — necesitás introducir una interface o hacer que el bot sea un `store.Player` con un flag especial.
- **Cómo se señala "es tu turno"**: no hay señal activa. El engine simplemente tiene `CurrentPlayerID` en el estado y el runtime espera una llamada HTTP a `POST /sessions/{id}/move`. Para el bot necesitás polling o un hook post-`ApplyMove` que detecte que el próximo jugador es un bot y dispara `RequestMove`.
- **Turn timer**: `turn_timer.go`, Redis TTL + keyspace notifications. El bot necesita responder antes de que expire el TTL. El `MaxThinkTime` del bot debe ser menor que el `TurnTimeoutSecs` de la sesión.
- **Estados en Postgres**: sí, JSON blobs. `session.State` es `[]byte`. Ya tenés `json.Unmarshal` en el runtime. El adapter puede reutilizar exactamente eso.
- **Love Letter visible vs hidden**: sí, `FilterState` ya existe y define exactamente qué es visible para cada jugador. El `Determinize` del adapter puede usar esa misma lógica para saber qué cartas son observables.

---

**Estimación por sesión:**

| Sesión | Trabajo | Complejidad |
|---|---|---|
| A — Interfaces & Clone | Definir `GameState`/`GameAdapter` interfaces, implementar `Clone()` para TicTacToe y Love Letter | Media — Love Letter tiene maps anidados, el deep copy es tedioso pero mecánico |
| B — Core MCTS | `node.go`, `mcts.go`, `engine.go` con mock adapter y tests | Alta — es el núcleo algorítmico, requiere cuidado con concurrencia |
| C — TicTacToe adapter | `GameAdapter` para TicTacToe, `Determinize` = identity, test bot vs bot | Baja — el estado es simple |
| D — Love Letter adapter | `GameAdapter` para Love Letter, `Determinize` con sampling de mano oculta | Alta — el sampling tiene que ser correcto para IS-MCTS |
| E — BotPlayer & integración | `BotPlayer`, hook en el runtime post-`ApplyMove`, `POST /rooms/{id}/bots` | Alta — requiere cambiar cómo el runtime maneja turnos, potencialmente introducir una interface de Player |
| F — Config & API | `BotConfig`, `PersonalityProfile`, env vars, `GET /bots/profiles` | Baja |

**Total estimado: 6 sesiones de tamaño similar a las que ya hicimos.**

El trabajo más delicado es la sesión E — integrar el bot en el runtime sin romper el flujo existente. Hoy el runtime es completamente reactivo (espera HTTP), y el bot necesita que sea también proactivo (detectar turno de bot y actuar). Hay dos approaches:

1. **Hook en `ApplyMove`**: después de aplicar un move, si el próximo jugador es un bot, llamar `BotManager.RequestMove` en una goroutine.
2. **Polling del BotManager**: un ticker que revisa sesiones activas con bots y actúa cuando es el turno del bot.

El approach 1 es más limpio pero requiere que el runtime conozca al `BotManager`. El approach 2 es más desacoplado pero introduce latencia y carga.

¿Querés arrancar con la sesión A?

Ahora tengo todo lo que necesito. Puedo responder las open questions restantes y proponer el diseño de la sesión A.

---

**Open questions resueltas:**

- **`PlayerID`**: `uuid.UUID` en el store. El bot necesita un `uuid.UUID` real como ID — lo generamos con `uuid.New()` al crear el bot.
- **`Player` interface**: struct concreto, sin interface. El bot necesita ser registrado como un `store.Player` en la DB para poder ser agregado a rooms via `AddPlayerToRoom`. Esto significa que el bot es un player real en la DB con un flag o naming convention que lo identifica como bot.
- **Turn hook**: no existe. La sesión E es efectivamente la más compleja.

---

**Diseño para la sesión A — interfaces y Clone:**

El doc propone `GameState interface { Clone() GameState }` y `GameAdapter` como interfaces nuevas en `internal/bot/`. El problema es que el engine ya tiene `engine.GameState` como struct concreto con `Data map[string]any`. Para el bot necesitamos un wrapper que implemente `Clone()` sin tocar el engine existente.

Propongo:

**`internal/bot/game.go`** — interfaces del bot, independientes del engine:

```go
// BotGameState wraps engine.GameState with a deep-copy operation required
// by the MCTS engine. MCTS branches the state thousands of times in parallel;
// Clone must produce a fully independent copy with no shared references.
type BotGameState interface {
    Clone() BotGameState
    // EngineState returns the underlying engine.GameState for use by adapters.
    EngineState() engine.GameState
}

// BotMove is an opaque move identifier. Must be usable as a map key.
// In practice this will be a JSON string of the move payload.
type BotMove string

// BotAdapter is the bridge between the MCTS engine and a specific game.
type BotAdapter interface {
    ValidMoves(s BotGameState) []BotMove
    ApplyMove(s BotGameState, m BotMove) BotGameState
    IsTerminal(s BotGameState) bool
    Result(s BotGameState, playerID engine.PlayerID) float64
    CurrentPlayer(s BotGameState) engine.PlayerID
    Determinize(s BotGameState, observer engine.PlayerID) BotGameState
}
```

**`internal/bot/state.go`** — implementación concreta de `BotGameState`:

```go
// concreteState wraps engine.GameState and implements BotGameState.
// Clone performs a deep copy of the Data map to prevent aliasing.
type concreteState struct {
    state engine.GameState
}

func NewBotState(s engine.GameState) BotGameState {
    return &concreteState{state: s}
}

func (c *concreteState) EngineState() engine.GameState { return c.state }

func (c *concreteState) Clone() BotGameState {
    // Deep copy via JSON round-trip — safe, consistent with how the runtime
    // already serializes/deserializes state to/from Postgres.
    b, _ := json.Marshal(c.state)
    var copy engine.GameState
    json.Unmarshal(b, &copy)
    return &concreteState{state: copy}
}
```

El `Clone()` via JSON round-trip es la misma estrategia que ya usa el runtime (`json.Marshal` + `json.Unmarshal`). Es ~10x más lento que un clone manual pero es correcto por definición — si el runtime puede sobrevivir con este encoding, el bot también. Para MCTS con 1000 iteraciones x 20 determinizaciones = 20000 clones, puede ser un bottleneck. Podemos optimizar después con clone manual por game.

**`BotMove` como `string`**: el payload de un move ya es `map[string]any` que se serializa a JSON. `BotMove` es simplemente ese JSON serializado — comparable como string, fácil de usar como map key.

---

¿Confirmás este diseño antes de que escriba los archivos?