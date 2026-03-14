/**
 * Represents the outcome of an operation that can either succeed or fail.
 *
 * The tuple shape is:
 *   - `[ErrorObject, null]` on failure
 *   - `[null, SuccessValue]` on success
 *
 * This makes errors values, forcing callers to explicitly handle them
 * instead of relying on thrown exceptions.
 *
 * @template S - The success value type.
 * @template E - The error object type. Must have a `reason` string discriminant.
 */
type Result<S, E extends { reason: string }> = [E, null] | [null, S];

/**
 * Wraps a success value into a `Result`.
 *
 * @example
 * return ok(user);
 */
export function ok<S>(result: S): Result<S, never> {
  return [null, result];
}

/**
 * Wraps an error object into a `Result`.
 * The `reason` field acts as a discriminant for exhaustive switch handling.
 *
 * @example
 * return error({ reason: "NOT_FOUND", id });
 */
export function error<const R extends string, E extends { reason: R }>(
  error: E
): Result<never, E> {
  return [error, null];
}

// ---------------------------------------------------------------------------
// Usage example
// ---------------------------------------------------------------------------

type GetUserError =
  | { reason: "NOT_FOUND"; id: string }
  | { reason: "UNAUTHORIZED" }
  | { reason: "DB_ERROR"; message: string };

declare function getUser(id: string): Result<{ name: string }, GetUserError>;

function handleGetUser(id: string) {
  const [err, user] = getUser(id);

  if (err) {
    switch (err.reason) {
      case "NOT_FOUND":
        console.error(`User ${err.id} does not exist`);
        break;

      case "UNAUTHORIZED":
        console.error("Access denied");
        break;

      case "DB_ERROR":
        console.error(`Database failure: ${err.message}`);
        break;

      default:
        // If any case above is removed, TypeScript will error here:
        // "Type 'X' is not assignable to type 'never'"
        err satisfies never;
    }

    return;
  }

  // `user` is fully narrowed here — no null check needed
  console.log(`Hello, ${user.name}`);
}
