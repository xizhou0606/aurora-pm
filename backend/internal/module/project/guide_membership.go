package project

import "context"

// GuideMembership is the minimal application-entry state; it does not disclose
// member lists or other applicants' requests.
type GuideMembership struct {
	Kind   string `json:"kind"`
	ID     uint   `json:"id"`
	Status string `json:"status"`
}
type GuideDetails struct {
	AccessGuide
	Memberships []GuideMembership `json:"memberships"`
}
type GuideMembershipReader interface {
	GuideMemberships(context.Context, JoinActor, AccessGuide) ([]GuideMembership, error)
}
