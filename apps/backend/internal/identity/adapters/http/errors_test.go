package http

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

func TestUserFacingDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "ErrUserBlocked",
			err:  application.ErrUserBlocked,
			want: detailUserBlocked,
		},
		{
			name: "ErrCodeSentTooRecently",
			err:  application.ErrCodeSentTooRecently,
			want: "Код отправлен слишком недавно",
		},
		{
			name: "ErrPhoneAlreadyTaken",
			err:  application.ErrPhoneAlreadyTaken,
			want: "Этот номер телефона уже используется",
		},
		{
			name: "ErrPhoneUnchanged",
			err:  application.ErrPhoneUnchanged,
			want: "Новый номер должен отличаться от текущего",
		},
		{
			name: "ErrTooManyAttempts",
			err:  domain.ErrTooManyAttempts,
			want: "Слишком много попыток",
		},
		{
			name: "ErrEmailDoesNotMatch",
			err:  application.ErrEmailDoesNotMatch,
			want: "Некорректные учётные данные",
		},
		{
			name: "ErrEmailAlreadyTaken",
			err:  application.ErrEmailAlreadyTaken,
			want: detailEmailTaken,
		},
		{
			name: "wrapped error resolves via errors.Is",
			err:  fmt.Errorf("send failed: %w", application.ErrUserBlocked),
			want: detailUserBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := userFacingDetail(tt.err)
			if !ok {
				t.Fatalf("userFacingDetail(%v) ok = false, want true", tt.err)
			}
			if got != tt.want {
				t.Fatalf("userFacingDetail(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestUserFacingDetail_UnknownErrorReturnsFalse(t *testing.T) {
	t.Parallel()

	got, ok := userFacingDetail(errors.New("something unexpected"))
	if ok {
		t.Fatalf("userFacingDetail(unknown) ok = true, want false")
	}
	if got != "" {
		t.Fatalf("userFacingDetail(unknown) = %q, want empty string", got)
	}
}

func TestUserFacingDetailOrDefault(t *testing.T) {
	t.Parallel()

	t.Run("known error returns its detail", func(t *testing.T) {
		t.Parallel()
		got := userFacingDetailOrDefault(application.ErrEmailAlreadyTaken, "fallback")
		if got != detailEmailTaken {
			t.Fatalf("got %q, want known error detail", got)
		}
	})

	t.Run("unknown error returns default", func(t *testing.T) {
		t.Parallel()
		got := userFacingDetailOrDefault(errors.New("boom"), "fallback detail")
		if got != "fallback detail" {
			t.Fatalf("got %q, want default", got)
		}
	})
}
