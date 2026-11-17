import type { PlatformUserQuery, PMScopeOptions } from '@/shared/api'
import type { QualitySceneOptionKind } from '../model/types'

export type QualityRawPage = {
  body: unknown
  items: unknown[]
  total: number
}

export type QualityUserPage = {
  items: unknown[]
  total: number
}

export type QualityClient = {
  list(query: Readonly<Record<string, unknown>>, signal?: AbortSignal): Promise<QualityRawPage>
  statistics(query: Readonly<Record<string, unknown>>, signal?: AbortSignal): Promise<QualityRawPage>
  dashboard(query: Readonly<Record<string, unknown>>, signal?: AbortSignal): Promise<unknown>
  detail(id: number, signal?: AbortSignal): Promise<unknown>
  activities(
    id: number,
    offset: number,
    limit: number,
    signal?: AbortSignal,
  ): Promise<QualityRawPage>
  comments(id: number, signal?: AbortSignal): Promise<QualityRawPage>
  users(query?: PlatformUserQuery, signal?: AbortSignal): Promise<QualityUserPage>
  scope(signal?: AbortSignal): Promise<PMScopeOptions>
  sceneOptions(kind: QualitySceneOptionKind, signal?: AbortSignal): Promise<unknown>
  createSceneOption(kind: QualitySceneOptionKind, name: string): Promise<unknown>
  deleteSceneOption(kind: QualitySceneOptionKind, id: number): Promise<void>
  create(body: unknown): Promise<unknown>
  patch(id: number, body: unknown): Promise<unknown>
  assign(id: number, body: unknown): Promise<unknown>
  batchAssign(ids: readonly number[], body: unknown): Promise<void>
  transition(id: number, body: unknown): Promise<unknown>
  createComment(id: number, body: string): Promise<unknown>
  deleteComment(id: number, commentId: number): Promise<void>
  remove(id: number): Promise<void>
  batchRemove(ids: readonly number[]): Promise<void>
  upload(requirementId: number, file: File): Promise<unknown>
}
