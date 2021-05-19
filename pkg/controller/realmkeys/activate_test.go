// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package realmkeys_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmkeys"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/sessions"
)

func TestRealmKeys_SubmitActivate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, harness.Config.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		t.Fatal(err)
	}
	c := realmkeys.New(harness.Config, harness.Database, harness.KeyManager, publicKeyCache, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleActivate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("no_form", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			User:        &database.User{},
			Realm:       &database.Realm{},
			Permissions: rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "", nil)
		handler.ServeHTTP(w, r)

		// shows original page with error flash
		if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})

	t.Run("invalid_key_id", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			User:        &database.User{},
			Realm:       &database.Realm{},
			Permissions: rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "", &url.Values{
			"id": []string{"1234567"},
		})
		handler.ServeHTTP(w, r)

		// shows original page with error flash
		if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := realm.CreateSigningKeyVersion(ctx, harness.Database, database.SystemTest); err != nil {
			t.Fatal(err)
		}
		list, err := realm.ListSigningKeys(harness.Database)
		if err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			User:        &database.User{},
			Realm:       realm,
			Permissions: rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "", &url.Values{
			"id": []string{fmt.Sprintf("%d", list[0].ID)},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Header().Get("Location"), "/realm/keys"; got != want {
			t.Errorf("expected %s to be %s", got, want)
		}
	})
}
