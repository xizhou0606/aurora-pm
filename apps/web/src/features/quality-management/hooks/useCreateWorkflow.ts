import { useCallback, useEffect } from 'react'

import type { QualityEnvironment } from '../components/environment'
import { qualityListPath } from '../model/route'
import { createPayload } from '../model/payloads'
import type { QualityBugDraft, QualityRouteState } from '../model/types'
import { useCreateMutation, useSceneOptionMutations, useUploadMutation } from './useMutations'
import { useQualitySceneOptionCatalogs, useQualityUsersQuery } from './useQueries'
import { useQualityOperationAccess } from './useOperationAccess'

export function useQualityCreateWorkflow(
  environment: QualityEnvironment,
  route: QualityRouteState,
) {
  const access = useQualityOperationAccess()
  const users = useQualityUsersQuery(environment)
  const canCreate = environment.allowed && access.canCreateBug
  const canUpload = access.canUploadMaterial(canCreate)
  const create = useCreateMutation(environment, canCreate)
  const upload = useUploadMutation(environment, canUpload)
  const sceneOptions = useQualitySceneOptionCatalogs(environment, access.canViewSceneOptions)
  const sceneOptionMutations = useSceneOptionMutations(environment, {
    canCreate: access.canCreateSceneOption,
    canDelete: access.canDeleteSceneOption,
  })

  useEffect(() => () => {
    create.reset()
    upload.reset()
    sceneOptionMutations.reset()
  }, [create.reset, sceneOptionMutations.reset, upload.reset])

  const submit = useCallback(async (draft: QualityBugDraft) => {
    await create.run(createPayload(draft))
    await environment.navigate(qualityListPath({
      ...route,
      kind: 'list',
    }), true)
  }, [create.run, environment, route])
  const cancel = useCallback(
    () => environment.navigate(qualityListPath(route)),
    [environment, route],
  )

  return {
    access: access.status === 'loading'
      ? 'loading' as const
      : canCreate
        ? 'allowed' as const
        : 'denied' as const,
    users,
    create,
    upload,
    sceneOptions,
    sceneOptionMutations,
    canUpload,
    submit,
    cancel,
  }
}
