package project

import (
	"context"
	"testing"
)

// Stub implementations for AccessService dependencies.

type stubUnitAccessReader struct {
	creators         map[uint]map[uint]uint              // workspace -> projectSet -> creator
	unitOwners       map[uint]map[uint]uint              // workspace -> planUnit -> owner
	unitParticipants map[uint]map[uint]map[uint]struct{} // workspace -> planUnit -> users
	unitToSet        map[uint]uint                       // planUnit -> projectSet
}

func (s *stubUnitAccessReader) ProjectCreatorID(ctx context.Context, workspaceID, projectSetID uint) (uint, error) {
	if s.creators == nil {
		return 0, nil
	}
	return s.creators[workspaceID][projectSetID], nil
}

func (s *stubUnitAccessReader) hasUnitRole(workspaceID, planUnitID, userID uint) bool {
	if s.unitOwners != nil && s.unitOwners[workspaceID][planUnitID] == userID {
		return true
	}
	if s.unitParticipants != nil {
		if _, ok := s.unitParticipants[workspaceID][planUnitID][userID]; ok {
			return true
		}
	}
	return false
}

func (s *stubUnitAccessReader) HasAnyPlanUnitAccess(ctx context.Context, workspaceID, projectSetID, userID uint) (bool, error) {
	if s.unitOwners != nil {
		for unitID, owner := range s.unitOwners[workspaceID] {
			if owner != userID {
				continue
			}
			if s.unitToSet[unitID] == projectSetID {
				return true, nil
			}
		}
	}
	if s.unitParticipants != nil {
		for unitID, users := range s.unitParticipants[workspaceID] {
			if _, ok := users[userID]; !ok {
				continue
			}
			if s.unitToSet[unitID] == projectSetID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *stubUnitAccessReader) HasPlanUnitAccess(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error) {
	if s.hasUnitRole(workspaceID, planUnitID, userID) {
		return true, nil
	}
	creator, err := s.ProjectCreatorID(ctx, workspaceID, s.unitToSet[planUnitID])
	if err != nil {
		return false, err
	}
	return creator == userID, nil
}

func (s *stubUnitAccessReader) CanManagePlanUnitMembers(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error) {
	if s.unitOwners != nil && s.unitOwners[workspaceID][planUnitID] == userID {
		return true, nil
	}
	creator, err := s.ProjectCreatorID(ctx, workspaceID, s.unitToSet[planUnitID])
	if err != nil {
		return false, err
	}
	return creator == userID, nil
}

func (s *stubUnitAccessReader) IsProjectSetMemberFact(ctx context.Context, workspaceID, projectSetID, userID uint) (bool, error) {
	creator, err := s.ProjectCreatorID(ctx, workspaceID, projectSetID)
	if err != nil {
		return false, err
	}
	if creator == userID {
		return true, nil
	}
	return s.HasAnyPlanUnitAccess(ctx, workspaceID, projectSetID, userID)
}

type stubFilterBuilder struct {
	fn func(ctx context.Context, table, projectColumn string, workspaceID, userID uint) (AccessFilter, error)
}

func (b *stubFilterBuilder) BuildMemberFilter(ctx context.Context, table, projectColumn string, workspaceID, userID uint) (AccessFilter, error) {
	if b.fn != nil {
		return b.fn(ctx, table, projectColumn, workspaceID, userID)
	}
	return AccessFilter{SQL: "(1=0)"}, nil
}

type stubAdminChecker struct {
	admins map[uint]map[uint]bool // workspaceID -> userID -> admin
}

func (c *stubAdminChecker) IsAccessAdministrator(ctx context.Context, workspaceID, userID uint) (bool, error) {
	if c.admins == nil {
		return false, nil
	}
	ws, ok := c.admins[workspaceID]
	if !ok {
		return false, nil
	}
	return ws[userID], nil
}

type stubActiveChecker struct {
	activeUsers map[uint]bool
}

func (c *stubActiveChecker) IsActive(ctx context.Context, workspaceID, userID uint) (bool, error) {
	if c.activeUsers == nil {
		return false, nil
	}
	return c.activeUsers[userID], nil
}

// Fixture: workspace 1, project set 777 created by 490, plan unit 31 owned by
// 7 with participant 20, plan unit 32 owned by 8; admin 99.
func makeStubAccessService(
	creators map[uint]map[uint]uint,
	admins map[uint]map[uint]bool,
	unitOwners map[uint]map[uint]uint,
	unitParticipants map[uint]map[uint]map[uint]struct{},
	unitToSet map[uint]uint,
	active map[uint]bool,
) *AccessService {
	svc, _ := NewAccessService(
		&stubAdminChecker{admins: admins},
		&stubActiveChecker{activeUsers: active},
		&stubUnitAccessReader{
			creators: creators, unitOwners: unitOwners,
			unitParticipants: unitParticipants, unitToSet: unitToSet,
		},
		&stubUnitAccessReader{
			creators: creators, unitOwners: unitOwners,
			unitParticipants: unitParticipants, unitToSet: unitToSet,
		},
		&stubFilterBuilder{},
	)
	return svc
}

func isSentinelP(err error, sentinel error) bool {
	if err == nil {
		return false
	}
	return err.Error() == sentinel.Error()
}

func testAccessService() *AccessService {
	return makeStubAccessService(
		map[uint]map[uint]uint{1: {777: 490}},
		map[uint]map[uint]bool{1: {99: true}},
		map[uint]map[uint]uint{1: {31: 7, 32: 8}},
		map[uint]map[uint]map[uint]struct{}{1: {31: {20: struct{}{}}}},
		map[uint]uint{31: 777, 32: 777},
		map[uint]bool{20: true, 99: true, 100: true, 490: true, 7: true, 8: true},
	)
}

func TestAccessServiceCanAccessProject(t *testing.T) {
	ctx := context.Background()
	svc := testAccessService()

	t.Run("creator gets access", func(t *testing.T) {
		ok, err := svc.CanAccessProject(ctx, 1, 777, 490)
		if err != nil || !ok {
			t.Fatalf("creator: ok=%v err=%v", ok, err)
		}
	})
	t.Run("unit owner gets access", func(t *testing.T) {
		ok, err := svc.CanAccessProject(ctx, 1, 777, 7)
		if err != nil || !ok {
			t.Fatalf("unit owner: ok=%v err=%v", ok, err)
		}
	})
	t.Run("unit participant gets access", func(t *testing.T) {
		ok, err := svc.CanAccessProject(ctx, 1, 777, 20)
		if err != nil || !ok {
			t.Fatalf("unit participant: ok=%v err=%v", ok, err)
		}
	})
	t.Run("admin gets access", func(t *testing.T) {
		ok, err := svc.CanAccessProject(ctx, 1, 777, 99)
		if err != nil || !ok {
			t.Fatalf("admin: ok=%v err=%v", ok, err)
		}
	})
	t.Run("non-member denied", func(t *testing.T) {
		ok, err := svc.CanAccessProject(ctx, 1, 777, 100)
		if err != nil || ok {
			t.Fatalf("non-member: ok=%v err=%v", ok, err)
		}
	})
	t.Run("zero userID denied", func(t *testing.T) {
		_, err := svc.CanAccessProject(ctx, 1, 777, 0)
		if !isSentinelP(err, ErrParticipantInvalid) {
			t.Fatalf("zero userID: err=%v", err)
		}
	})
	t.Run("inactive user denied", func(t *testing.T) {
		ok, _ := svc.CanAccessProject(ctx, 1, 777, 999)
		if ok {
			t.Fatal("inactive user allowed")
		}
	})
	t.Run("admin allowed even without workspace membership", func(t *testing.T) {
		// Simulates a platform super_admin: IsAccessAdministrator returns true
		// while IsActive (workspace_members) would return false for this user.
		svcAdminOnly := makeStubAccessService(
			map[uint]map[uint]uint{1: {777: 490}},
			map[uint]map[uint]bool{1: {99: true}},
			map[uint]map[uint]uint{1: {31: 7}},
			nil,
			map[uint]uint{31: 777},
			map[uint]bool{20: true, 100: true, 490: true, 7: true},
		)
		ok, err := svcAdminOnly.CanAccessProject(ctx, 1, 777, 99)
		if err != nil || !ok {
			t.Fatalf("admin without workspace membership: ok=%v err=%v", ok, err)
		}
	})
}

func TestAccessServiceCanAccessPlanUnit(t *testing.T) {
	ctx := context.Background()
	svc := testAccessService()

	t.Run("unit owner passes", func(t *testing.T) {
		ok, err := svc.CanAccessPlanUnit(ctx, 1, 31, 7)
		if err != nil || !ok {
			t.Fatalf("unit owner: ok=%v err=%v", ok, err)
		}
	})
	t.Run("unit participant passes", func(t *testing.T) {
		ok, err := svc.CanAccessPlanUnit(ctx, 1, 31, 20)
		if err != nil || !ok {
			t.Fatalf("unit participant: ok=%v err=%v", ok, err)
		}
	})
	t.Run("set creator passes", func(t *testing.T) {
		ok, err := svc.CanAccessPlanUnit(ctx, 1, 31, 490)
		if err != nil || !ok {
			t.Fatalf("set creator: ok=%v err=%v", ok, err)
		}
	})
	t.Run("other unit owner denied", func(t *testing.T) {
		ok, err := svc.CanAccessPlanUnit(ctx, 1, 31, 8)
		if err != nil || ok {
			t.Fatalf("other unit owner: ok=%v err=%v", ok, err)
		}
	})
	t.Run("unit participant cannot manage members", func(t *testing.T) {
		ok, err := svc.CanManagePlanUnitMembers(ctx, 1, 31, 20)
		if err != nil || ok {
			t.Fatalf("participant manage: ok=%v err=%v", ok, err)
		}
	})
	t.Run("unit owner can manage members", func(t *testing.T) {
		ok, err := svc.CanManagePlanUnitMembers(ctx, 1, 31, 7)
		if err != nil || !ok {
			t.Fatalf("unit owner manage: ok=%v err=%v", ok, err)
		}
	})
	t.Run("set creator can manage members", func(t *testing.T) {
		ok, err := svc.CanManagePlanUnitMembers(ctx, 1, 31, 490)
		if err != nil || !ok {
			t.Fatalf("set creator manage: ok=%v err=%v", ok, err)
		}
	})
	t.Run("admin can manage members", func(t *testing.T) {
		ok, err := svc.CanManagePlanUnitMembers(ctx, 1, 31, 99)
		if err != nil || !ok {
			t.Fatalf("admin manage: ok=%v err=%v", ok, err)
		}
	})
}

func TestAccessServiceVerifyProjectMembers(t *testing.T) {
	ctx := context.Background()
	svc := testAccessService()

	t.Run("creator verified", func(t *testing.T) {
		err := svc.VerifyProjectMembers(ctx, 1, 777, []uint{490})
		if err != nil {
			t.Fatalf("creator: err=%v", err)
		}
	})
	t.Run("unit owner verified", func(t *testing.T) {
		err := svc.VerifyProjectMembers(ctx, 1, 777, []uint{7})
		if err != nil {
			t.Fatalf("unit owner: err=%v", err)
		}
	})
	t.Run("unit participant verified", func(t *testing.T) {
		err := svc.VerifyProjectMembers(ctx, 1, 777, []uint{20})
		if err != nil {
			t.Fatalf("unit participant: err=%v", err)
		}
	})
	t.Run("non-member rejected", func(t *testing.T) {
		err := svc.VerifyProjectMembers(ctx, 1, 777, []uint{100})
		if !isSentinelP(err, ErrParticipantNotOwner) {
			t.Fatalf("non-member: err=%v", err)
		}
	})
	t.Run("zero inputs", func(t *testing.T) {
		err := svc.VerifyProjectMembers(ctx, 0, 777, []uint{100})
		if !isSentinelP(err, ErrParticipantInvalid) {
			t.Fatalf("zero workspace: err=%v", err)
		}
	})
}

func TestAccessServiceRequireProjectAccess(t *testing.T) {
	ctx := context.Background()
	svc := testAccessService()

	t.Run("unit participant passes", func(t *testing.T) {
		err := svc.RequireProjectAccess(ctx, 1, 777, 20)
		if err != nil {
			t.Fatalf("unit participant: err=%v", err)
		}
	})
	t.Run("non-member blocked", func(t *testing.T) {
		err := svc.RequireProjectAccess(ctx, 1, 777, 100)
		if !isSentinelP(err, ErrProjectAccessNotFound) {
			t.Fatalf("non-member: err=%v", err)
		}
	})
}

func TestAccessServiceBuildMemberFilter(t *testing.T) {
	ctx := context.Background()
	admins := map[uint]map[uint]bool{1: {99: true}}
	active := map[uint]bool{99: true, 20: true}
	filter := AccessFilter{SQL: "EXISTS (SELECT 1)"}

	filterBuilder := &stubFilterBuilder{
		fn: func(ctx context.Context, table, projectColumn string, workspaceID, userID uint) (AccessFilter, error) {
			return filter, nil
		},
	}

	newSvc := func(admins map[uint]map[uint]bool, active map[uint]bool) *AccessService {
		svc, _ := NewAccessService(
			&stubAdminChecker{admins: admins},
			&stubActiveChecker{activeUsers: active},
			&stubUnitAccessReader{},
			&stubUnitAccessReader{},
			filterBuilder,
		)
		return svc
	}

	t.Run("admin returns empty filter", func(t *testing.T) {
		f, err := newSvc(admins, active).BuildMemberFilter(ctx, "t", "c", 1, 99)
		if err != nil || f.SQL != "" {
			t.Fatalf("admin: filter=%+v err=%v", f, err)
		}
	})
	t.Run("non-admin returns member filter", func(t *testing.T) {
		f, err := newSvc(nil, active).BuildMemberFilter(ctx, "t", "c", 1, 20)
		if err != nil || f.SQL != "EXISTS (SELECT 1)" {
			t.Fatalf("non-admin: filter=%+v err=%v", f, err)
		}
	})
	t.Run("zero inputs", func(t *testing.T) {
		_, err := newSvc(nil, active).BuildMemberFilter(ctx, "t", "c", 0, 20)
		if !isSentinelP(err, ErrParticipantInvalid) {
			t.Fatalf("zero workspace: err=%v", err)
		}
	})
}
