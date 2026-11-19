// Package user resolves uids and gids for privilege dropping.
package user

import (
	"os/user"
	"strconv"
)

// Lookup returns the uid/gid of a user by name.
//
// It uses os/user so it follows the platform's user backend: /etc/passwd for
// static binaries, NSS (LDAP, sssd, …) when cgo is available. That keeps the
// tool usable in environments where users do not live in /etc/passwd.
func Lookup(name string) (uid, gid uint32, ok bool) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, false
	}
	u32, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, false
	}
	g32, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return 0, 0, false
	}
	return uint32(u32), uint32(g32), true
}

// Current returns the uid/gid of the running process.
func Current() (uid, gid uint32, ok bool) {
	u, err := user.Current()
	if err != nil {
		return 0, 0, false
	}
	u32, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, false
	}
	g32, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return 0, 0, false
	}
	return uint32(u32), uint32(g32), true
}
