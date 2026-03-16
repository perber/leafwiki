export function deferStateUpdate(fn: () => void) {
  Promise.resolve().then(fn)
}
