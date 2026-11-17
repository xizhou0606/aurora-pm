package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
)

type qualityNoteLengthError struct {
	Field string
	Limit int
}

func (e *qualityNoteLengthError) Error() string {
	return fmt.Sprintf("%s cannot exceed %d visible characters", e.Field, e.Limit)
}

func validateQualityNoteLength(field, value string) error {
	if quality.RichTextRuneCount(value) <= qualityBugNoteMaxRunes {
		return nil
	}
	return &qualityNoteLengthError{Field: field, Limit: qualityBugNoteMaxRunes}
}

func respondQualityNoteLengthError(c *gin.Context, err error) bool {
	var validation *qualityNoteLengthError
	if !errors.As(err, &validation) {
		return false
	}
	code := apiErrorQualityResolutionNoteTooLong
	switch validation.Field {
	case "assignment_note":
		code = apiErrorQualityAssignmentNoteTooLong
	case "regression_note":
		code = apiErrorQualityRegressionNoteTooLong
	}
	failCode(c, http.StatusUnprocessableEntity, code, validation.Error())
	return true
}
