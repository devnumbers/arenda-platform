package application

// UpdateProfileCommand carries the optional personal data fields for a profile update.
type UpdateProfileCommand struct {
	Name       *string
	Surname    *string
	Patronymic *string
	Email      *string
	Timezone   *string
}
