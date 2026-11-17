package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
)

var errPMUserRegistryUnavailable = errors.New("PM user registry unavailable")

// pmUserReferenceInactiveError marks a business reference to a user ID that
// matches no active PM user. Transport maps it to a machine-readable code so
// the web client can render a specific remediation message (the safe-error
// design never displays free-text server messages).
type pmUserReferenceInactiveError struct {
	referenceID uint
}

func (e *pmUserReferenceInactiveError) Error() string {
	return fmt.Sprintf("PM user reference ID %d is not active", e.referenceID)
}

// PM users are the authoritative people registry. Business references resolve
// only users.id; external import identifiers never alias online identities.
func (s *Server) findActivePMUsersByReferenceIDs(ids []uint) (map[uint]identitygorm.User, error) {
	return findActivePMUsersByReferenceIDs(s.db, ids)
}

// Business records use PM internal user IDs after the identity cutover.
// Display remains independent from membership, but never aliases external IDs.
func (s *Server) findActivePMUsersByFactIDs(ids []uint) (map[uint]identitygorm.User, error) {
	return findActivePMUsersByReferenceIDs(s.db, ids)
}

func findActivePMUsersByReferenceIDs(db *gorm.DB, ids []uint) (map[uint]identitygorm.User, error) {
	ids = mergeSortedUserIDs(ids)
	users := make(map[uint]identitygorm.User, len(ids))
	if len(ids) == 0 {
		return users, nil
	}

	var rows []identitygorm.User
	if err := db.Model(&identitygorm.User{}).
		Where("users.active = ?", true).
		Where("users.id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", errPMUserRegistryUnavailable, err)
	}
	return pmUsersByReferenceID(rows)
}

func pmUsersByReferenceID(rows []identitygorm.User) (map[uint]identitygorm.User, error) {
	users := make(map[uint]identitygorm.User, len(rows))
	for _, user := range rows {
		users[user.ID] = user
	}
	return users, nil
}

func (s *Server) activePMUserByReferenceID(referenceID uint) (identitygorm.User, error) {
	users, err := s.findActivePMUsersByReferenceIDs([]uint{referenceID})
	if err != nil {
		return identitygorm.User{}, err
	}
	user, found := users[referenceID]
	if !found {
		return identitygorm.User{}, &pmUserReferenceInactiveError{referenceID: referenceID}
	}
	return user, nil
}

func (s *Server) validateActivePMUserReferences(_ *gin.Context, ids []uint) error {
	ids = mergeSortedUserIDs(ids)
	users, err := s.findActivePMUsersByReferenceIDs(ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, found := users[id]; !found {
			return &pmUserReferenceInactiveError{referenceID: id}
		}
	}
	return nil
}

func respondPMUserRegistryValidationError(c *gin.Context, err error) {
	if errors.Is(err, errPMUserRegistryUnavailable) {
		dbFail(c)
		return
	}
	var inactive *pmUserReferenceInactiveError
	if errors.As(err, &inactive) {
		failCode(c, http.StatusBadRequest, "PM_USER_REFERENCE_INACTIVE", err.Error())
		return
	}
	bad(c, err.Error())
}

func pmUserRows(users []identitygorm.User) []gin.H {
	rows := make([]gin.H, 0, len(users))
	for _, user := range users {
		rows = append(rows, pmUserJSON(user))
	}
	return rows
}
