package application

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestParticipantService_ParticipantCarriesPhotoUrl proves a registered
// participant carries their profile photo's same-origin streaming path
// (ADR 0065, решение владельца #1286), built from the user's photo key, and
// a pending email row carries none.
func TestParticipantService_ParticipantCarriesPhotoUrl(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)
	u1 := f.users.users[f.u1]
	u1.PhotoKey = new("photos/abc.jpg")
	f.users.users[f.u1] = u1

	participants, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants error = %v", err)
	}

	var registered, pending *Participant
	for i := range participants {
		p := participants[i]
		switch p.UserID {
		case f.u1:
			registered = &participants[i]
		case uuid.UUID{}:
			pending = &participants[i]
		}
	}
	if registered == nil {
		t.Fatal("registered participant not found")
	}
	if pending == nil {
		t.Fatal("pending participant not found")
	}
	if !strings.HasSuffix(registered.PhotoURL, "/api/users/"+f.u1.String()+"/photo") {
		t.Fatalf("registered photoUrl = %q, want the /users streaming path", registered.PhotoURL)
	}
	if pending.PhotoURL != "" {
		t.Fatalf("pending photoUrl = %q, want empty (no account, no photo)", pending.PhotoURL)
	}
}

// TestParticipantService_ParticipantWithoutPhotoHasEmptyPath proves a user
// without a photo key yields the empty path — the nullable contract's null.
func TestParticipantService_ParticipantWithoutPhotoHasEmptyPath(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)

	participants, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants error = %v", err)
	}
	for _, p := range participants {
		if p.PhotoURL != "" {
			t.Fatalf("participant %s photoUrl = %q, want empty without a photo key", p.UserID, p.PhotoURL)
		}
	}
}

// TestParticipantService_LegsCarryTypeAndPhoto proves each access leg is
// self-sufficient for the avatar: the property's type (the placeholder
// glyph's key, остаток #1244 п.3) and the photo's streaming path come from
// the scope row — no /properties join on the client, no BoldHome on a cold
// cache.
func TestParticipantService_LegsCarryTypeAndPhoto(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)
	f.scope[0].Type = "house"
	f.scope[0].PhotoPath = "/api/properties/" + f.p1.String() + "/photo"
	f.read.props = f.scope

	participants, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants error = %v", err)
	}

	var u1 *Participant
	for i := range participants {
		if participants[i].UserID == f.u1 {
			u1 = &participants[i]
		}
	}
	if u1 == nil {
		t.Fatal("participant not found")
	}
	var p1Leg, p2Leg *ParticipantProperty
	for i := range u1.Properties {
		switch u1.Properties[i].PropertyID {
		case f.p1:
			p1Leg = &u1.Properties[i]
		case f.p2:
			p2Leg = &u1.Properties[i]
		}
	}
	if p1Leg == nil || p2Leg == nil {
		t.Fatalf("legs = %+v, want one per scoped property", u1.Properties)
	}
	if p1Leg.Type != "house" || p1Leg.PhotoURL != "/api/properties/"+f.p1.String()+"/photo" {
		t.Fatalf("p1 leg = (type %q, photo %q), want (house, streaming path)", p1Leg.Type, p1Leg.PhotoURL)
	}
	if p2Leg.Type != "" || p2Leg.PhotoURL != "" {
		t.Fatalf("p2 leg = (type %q, photo %q), want empty (the fixture's bare property)", p2Leg.Type, p2Leg.PhotoURL)
	}
}
