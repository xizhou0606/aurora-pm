import type { ChartModel, CompositionModel, DashboardData, VersionMatrixModel } from '../types'
import { compositionModel } from './composition'
import { commandRadarModel } from './radar'
import { completionCapsuleModel, ownerLoadBarModel } from './ranking'
import { trendChart } from './trend'
import { versionMatrixModel } from './version-matrix'

export type OverviewModels = {
  trend: ChartModel
  version: ChartModel
  requirement: ChartModel
  owner: ChartModel
  composition: CompositionModel
  matrix: VersionMatrixModel
  healthRadar: ChartModel
}

export function overviewModels(data: DashboardData): OverviewModels {
  return {
    trend: trendChart(data),
    version: completionCapsuleModel('version', data.version_progress),
    requirement: completionCapsuleModel('requirement', data.requirement_progress),
    owner: ownerLoadBarModel(data.owner_board),
    composition: compositionModel(data),
    matrix: versionMatrixModel(data),
    healthRadar: commandRadarModel(data),
  }
}
