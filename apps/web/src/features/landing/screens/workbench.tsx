import type { ReactNode } from 'react'

import { SHELL_NAVIGATION_GROUPS } from '@/app/shell/registry'
import { BrandMark } from '@/shared/ui/brand/brand-mark'
import { copy } from '@/shared/copy'
import {
  LANDING_USER_LABEL,
  LANDING_WORKSPACE_LABEL,
} from '../live/fixtures'

export type LandingWorkbenchId =
  | 'inbox'
  | 'tasks'
  | 'workflow'
  | 'command'
  | 'quality'
  | 'requirement-detail'
  | 'requirement-todo'

const sceneNavId: Record<LandingWorkbenchId, string> = {
  inbox: 'inbox',
  tasks: 'workItems',
  workflow: 'workflows',
  command: 'biScreen',
  quality: 'quality',
  'requirement-detail': 'requirements',
  'requirement-todo': 'requirements',
}

export function LandingWorkbenchFallback({ scene }: { scene: LandingWorkbenchId }) {
  const active = sceneNavId[scene]
  return (
    <LandingWorkbenchChrome>
      <div aria-hidden="true" className="landing-workbench" data-slot="landing-workbench-shell">
        <div className="landing-workbench-top">
          <BrandMark className="landing-workbench-mark" size={26} />
          <strong>{copy.app.productName}</strong>
          <span className="landing-workbench-space">{LANDING_WORKSPACE_LABEL}</span>
          <span className="landing-workbench-user">{LANDING_USER_LABEL}</span>
        </div>
        <div className="landing-workbench-body">
          <nav className="landing-workbench-nav">
            {SHELL_NAVIGATION_GROUPS.map((group) => (
              <section key={group.id}>
                <h2>{group.label}</h2>
                {group.items.map((item) => (
                  <span className={item.id === active ? 'is-active' : undefined} key={item.id}>{item.label}</span>
                ))}
              </section>
            ))}
          </nav>
          <div className="landing-workbench-main landing-workbench-main--pending">
            <i /><i /><i /><i />
          </div>
        </div>
      </div>
    </LandingWorkbenchChrome>
  )
}

export function LandingWorkbenchChrome({ children }: { children?: ReactNode }) {
  return (
    <div className="landing-screen">
      <div className="landing-screen-frame">
        <div aria-hidden="true" className="landing-screen-chrome">
          <span className="landing-screen-lights">
            <i data-light="close" />
            <i data-light="minimize" />
            <i data-light="zoom" />
          </span>
        </div>
        <div className="landing-screen-viewport">
          <div className="landing-screen-stage" data-slot="landing-live-frame">
            {children}
          </div>
        </div>
      </div>
    </div>
  )
}
