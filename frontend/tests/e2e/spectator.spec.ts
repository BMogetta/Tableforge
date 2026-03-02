// Spectator mode tests — blocked pending backend endpoints and frontend UI.
//
// The store methods (AddSpectator, RemoveSpectator, ListSpectators) exist in
// pg_store.go but there are no HTTP endpoints or frontend components yet.
// Uncomment and implement these tests once the feature is built.
//
// import { test, expect } from '@playwright/test'
//
// test.describe('Spectator mode', () => {
//   test('spectator can watch an in-progress game', async ({ browser }) => {
//     // 1. P1 and P2 start a game.
//     // 2. P3 (a third context) navigates to /game/:id directly.
//     // 3. Assert P3 can see the board state but cannot make moves.
//     // 4. P1 makes a move — assert P3 sees the update via WS.
//   })
//
//   test('spectator count is shown to players', async ({ browser }) => {
//     // 1. P1 and P2 start a game.
//     // 2. P3 joins as spectator.
//     // 3. Assert spectator count UI updates for P1 and P2.
//     // 4. P3 leaves — assert count decrements.
//   })
// })