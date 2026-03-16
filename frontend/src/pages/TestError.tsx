// Renders a component that throws during render to test the ErrorBoundary.
// Only accessible when VITE_TEST_MODE=true.
export default function TestError() {
  const shouldThrow = true
  if (shouldThrow) {
    throw new Error('Test error triggered intentionally')
  }
  return null
}
