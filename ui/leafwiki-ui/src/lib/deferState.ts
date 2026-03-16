export function deferStateUpdate(fn: () => void) {
  queueMicrotask(fn)
}
