package goversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVersion(t *testing.T) {
	for _, td := range []struct {
		input   string
		want    string
		wantErr bool
	}{
		{
			input: "go1.16rc1",
		},
		{
			input: "1.16rc1",
			want:  "go1.16rc1",
		},
		{
			input:   "v1.16rc1",
			wantErr: true,
		},
		{
			input:   "",
			wantErr: true,
		},
		{
			input: "go1",
		},
		{
			input: "go1.15",
		},
		{
			input:   "go1.15.x",
			wantErr: true,
		},
		{
			input: "go2rc1",
		},
	} {
		t.Run(td.input, func(t *testing.T) {
			got, err := NewVersion(td.input)
			if td.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			want := td.want
			if want == "" {
				want = td.input
			}
			require.Equal(t, want, got.String())
		})
	}
}

func checkConstraint(t *testing.T, constraint, version string) bool {
	t.Helper()
	c, err := NewConstraints(constraint)
	require.NoError(t, err)
	v, err := NewVersion(version)
	require.NoError(t, err)
	return c.Check(v)
}

func TestConstraints_Check(t *testing.T) {
	assert.True(t, checkConstraint(t, "1.2beta1", "1.2beta1"))
	assert.False(t, checkConstraint(t, "1.2beta1", "1.2"))
}

func TestNewConstraints(t *testing.T) {
	for _, td := range []struct {
		s, want string
	}{
		{"1.2beta1", "1.2.0-beta1"},
		{"^1.2beta1", "^1.2.0-beta1"},
		{"asdf", ""},
		{"1.x", "1.x"},
	} {
		t.Run(td.s, func(t *testing.T) {
			got, err := NewConstraints(td.s)
			if td.want == "" {
				require.EqualError(t, err, ErrInvalidConstraint.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, td.want, got.constraints.String())
		})
	}
}

func Test_go2semverRange(t *testing.T) {
	require.Equal(t, "1.2.0-beta1", go2SemverRange("1.2beta1"))
}
