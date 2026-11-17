export type DeliveryRoadmapActions = {
  editLane?: (laneId: number) => void
  quickEdit?: (laneId: number, nodeKey: string) => void
}