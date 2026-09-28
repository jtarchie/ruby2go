//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbEtcPasswd builds an Etc::Passwd from an os/user.User; passwd/shell/change/uclass/expire have no os/user equivalent and are always nil (decision 63).
func rbEtcPasswd(u *user.User) *Etc_Passwd {
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	return NewEtc_Passwd(String(u.Username), nil, Integer(uid), Integer(gid), String(u.Name), String(u.HomeDir), nil, nil, nil, nil)
}
