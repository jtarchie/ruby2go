//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"os/user"
	"strconv"
)

// rbEtcPasswd builds an Etc::Passwd from an os/user.User; passwd/shell/change/uclass/expire have no os/user equivalent and are always nil (decision 63).
func rbEtcPasswd(u *user.User) *Etc_Passwd {
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	return NewEtc_Passwd(String(u.Username), nil, Integer(uid), Integer(gid), String(u.Name), String(u.HomeDir), nil, nil, nil, nil)
}
