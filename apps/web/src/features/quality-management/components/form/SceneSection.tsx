import { QUALITY_COPY } from '@/shared/copy'
import type {
  QualityBugDraft,
  QualitySceneOptionCatalogs,
  QualitySceneOptionMutations,
} from '../../model/types'
import { SceneOptionField } from './EnvironmentField'

export function SceneSection({
  draft,
  sceneOptions,
  sceneOptionMutations,
  onUpdate,
}: {
  draft: QualityBugDraft
  sceneOptions: QualitySceneOptionCatalogs
  sceneOptionMutations?: QualitySceneOptionMutations
  onUpdate: <TKey extends keyof QualityBugDraft>(key: TKey, value: QualityBugDraft[TKey]) => void
}) {
  return (
    <section className="grid gap-3 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">{QUALITY_COPY.form.scene}</h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <SceneOptionField
          kind="environment"
          id="quality-form-environment"
          label={QUALITY_COPY.form.environment}
          placeholder={QUALITY_COPY.form.environmentPlaceholder}
          value={draft.foundEnvironment}
          state={sceneOptions.environment}
          mutations={sceneOptionMutations}
          onValueChange={(value) => onUpdate('foundEnvironment', value)}
        />
        <SceneOptionField
          kind="operating_system"
          id="quality-form-operating-system"
          label={QUALITY_COPY.form.operatingSystem}
          placeholder={QUALITY_COPY.form.operatingSystem}
          value={draft.operatingSystem}
          state={sceneOptions.operating_system}
          mutations={sceneOptionMutations}
          onValueChange={(value) => onUpdate('operatingSystem', value)}
        />
        <SceneOptionField
          kind="browser"
          id="quality-form-browser"
          label={QUALITY_COPY.form.browser}
          placeholder={QUALITY_COPY.form.browser}
          value={draft.browser}
          state={sceneOptions.browser}
          mutations={sceneOptionMutations}
          onValueChange={(value) => onUpdate('browser', value)}
        />
      </div>
    </section>
  )
}