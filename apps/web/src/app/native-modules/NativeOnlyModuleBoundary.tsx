import type { PMBusinessModuleId } from '@/shared/config'
import { LocalBoundary, localNativeModuleLoader } from './LocalBoundary'
import { NativeModuleError } from './BoundaryView'

export function NativeOnlyModuleBoundary({ moduleId }: { moduleId: PMBusinessModuleId }) {
  const loader = localNativeModuleLoader(moduleId)

  if (!loader) return <NativeModuleError reason="load" />
  return (
    <LocalBoundary
      moduleId={moduleId}
      loader={loader}
    />
  )
}
