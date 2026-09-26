package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"supportchat/business/models"
)

var (
	ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}
	cat = models.User{ID: "customer:cat", Name: "Cat", Role: models.RoleCustomer}
	bob = models.User{ID: "agent:bob", Name: "Bob", Role: models.RoleAgent}
)

func TestCaseCanView(t *testing.T) {
	c := models.Case{ID: "c1", CustomerID: ann.ID}

	assert.True(t, c.CanView(ann), "owner can view")
	assert.True(t, c.CanView(bob), "any agent can view")
	assert.False(t, c.CanView(cat), "another customer cannot view")
	assert.False(t, c.CanView(models.User{ID: ann.ID, Role: "admin"}), "unknown role cannot view")
}

func TestCaseHasParticipant(t *testing.T) {
	c := models.Case{Participants: []models.Participant{{UserID: ann.ID}, {UserID: bob.ID}}}

	assert.True(t, c.HasParticipant(bob.ID))
	assert.False(t, c.HasParticipant("agent:dan"))
}

func TestSubjectFrom(t *testing.T) {
	assert.Equal(t, "How do I reset my password?", models.SubjectFrom("How do I\n  reset my password?"))
	assert.Equal(t, strings.Repeat("ก", 80), models.SubjectFrom(strings.Repeat("ก", 81)))
}

func TestValidRoleAndStatus(t *testing.T) {
	assert.True(t, models.RoleAgent.Valid())
	assert.False(t, models.Role("admin").Valid())
	assert.True(t, models.StatusClosed.Valid())
	assert.False(t, models.CaseStatus("archived").Valid())
}
