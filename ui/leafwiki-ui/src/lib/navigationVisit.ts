type NavigationVisitLocation = {
  key?: string
  pathname: string
  state: unknown
}

const NAVIGATION_VISIT_ID_KEY = 'leafwikiVisitId'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function createVisitId(): string {
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return crypto.randomUUID()
  }

  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function createNavigationVisitState(state?: unknown) {
  const nextState = isRecord(state) ? { ...state } : {}
  nextState[NAVIGATION_VISIT_ID_KEY] = createVisitId()
  return nextState
}

export function getNavigationVisitKey(location: NavigationVisitLocation) {
  if (isRecord(location.state)) {
    const visitId = location.state[NAVIGATION_VISIT_ID_KEY]
    if (typeof visitId === 'string' && visitId.length > 0) {
      return visitId
    }
  }

  return location.key || location.pathname
}
