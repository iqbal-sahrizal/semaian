package main

import (
	"testing"

	"github.com/semaian/platform/services/api/internal/modules/knowledge"
	"github.com/semaian/platform/services/api/internal/platform/auth"
)

func TestEnvEditorialRoleResolver(t *testing.T) {
	t.Run("valid independent roles and unknown user", func(t *testing.T) {
		t.Setenv("EDITORIAL_SUPER_ADMINS", "sub-admin")
		t.Setenv("EDITORIAL_REVIEWERS", "sub-rev")
		t.Setenv("EDITORIAL_EDITORS", "sub-ed, , ")

		res, err := newEnvEditorialRoleResolver()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := res.ResolveEditorialRole(auth.Actor{Subject: "sub-admin"}); got != knowledge.RoleSuperAdmin {
			t.Fatalf("expected RoleSuperAdmin, got %v", got)
		}
		if got := res.ResolveEditorialRole(auth.Actor{Subject: "sub-rev"}); got != knowledge.RoleKnowledgeReviewer {
			t.Fatalf("expected RoleKnowledgeReviewer, got %v", got)
		}
		if got := res.ResolveEditorialRole(auth.Actor{Subject: "sub-ed"}); got != knowledge.RoleKnowledgeEditor {
			t.Fatalf("expected RoleKnowledgeEditor, got %v", got)
		}
		if got := res.ResolveEditorialRole(auth.Actor{Subject: "unknown-user"}); got != knowledge.RoleReader {
			t.Fatalf("expected RoleReader for unknown user, got %v", got)
		}
		if got := res.ResolveEditorialRole(auth.Actor{Subject: ""}); got != knowledge.RoleReader {
			t.Fatalf("expected RoleReader for empty subject, got %v", got)
		}
	})

	t.Run("conflicting roles fail closed at startup", func(t *testing.T) {
		cases := []struct {
			name   string
			admins string
			revs   string
			eds    string
		}{
			{"editor and reviewer", "", "sub-x", "sub-x"},
			{"reviewer and super_admin", "sub-x", "sub-x", ""},
			{"all three", "sub-x", "sub-x", "sub-x"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Setenv("EDITORIAL_SUPER_ADMINS", tc.admins)
				t.Setenv("EDITORIAL_REVIEWERS", tc.revs)
				t.Setenv("EDITORIAL_EDITORS", tc.eds)
				if _, err := newEnvEditorialRoleResolver(); err == nil {
					t.Fatalf("expected error for conflicting role assignment (%s)", tc.name)
				}
			})
		}
	})
}
