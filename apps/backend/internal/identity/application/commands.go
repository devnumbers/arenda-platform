package application

// UpdateProfileCommand carries the optional personal data fields for a profile update.
type UpdateProfileCommand struct {
	Name       *string
	Surname    *string
	Patronymic *string
	Email      *string
	Timezone   *string
}

// ChangedFields lists the names of the fields a command changes. Only field
// names are audited, never their values. Co-located with the struct definition
// so adding a new field is less likely to drift from this mirror.
func (c UpdateProfileCommand) ChangedFields() []string {
	fields := make([]string, 0, 5)
	if c.Name != nil {
		fields = append(fields, "name")
	}
	if c.Surname != nil {
		fields = append(fields, "surname")
	}
	if c.Patronymic != nil {
		fields = append(fields, "patronymic")
	}
	if c.Email != nil {
		fields = append(fields, "email")
	}
	if c.Timezone != nil {
		fields = append(fields, "timezone")
	}
	return fields
}
