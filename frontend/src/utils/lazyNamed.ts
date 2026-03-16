import { lazy } from 'react'

export function lazyNamed<T extends Record<string, unknown>, K extends keyof T>(
  importer: () => Promise<T>,
  name: K
) {
  return lazy(() =>
    importer().then((module) => ({
      default: module[name] as React.ComponentType
    }))
  )
}