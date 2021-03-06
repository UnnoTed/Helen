package authority

type AuthAction int

type AuthRole int

var permissions = make(map[AuthRole]map[AuthAction]bool)

func (role AuthRole) Allow(action AuthAction) AuthRole {
	amap, ok := permissions[role]
	if !ok {
		amap = make(map[AuthAction]bool)
		permissions[role] = amap
	}

	amap[action] = true
	return role
}

func (role AuthRole) Disallow(action AuthAction) AuthRole {
	amap, ok := permissions[role]
	if !ok {
		amap = make(map[AuthAction]bool)
		permissions[role] = amap
	}

	amap[action] = false
	return role
}

func (myrole AuthRole) Inherit(otherrole AuthRole) AuthRole {
	mymap, ok := permissions[myrole]
	if !ok {
		mymap = make(map[AuthAction]bool)
		permissions[myrole] = mymap
	}

	othermap, otherok := permissions[otherrole]
	if !otherok {
		return myrole
	}

	for entry, val := range othermap {
		mymap[entry] = val
	}

	return myrole
}

func (role AuthRole) Can(action AuthAction) bool {
	mymap, ok := permissions[role]
	return ok && mymap[action]
}

func Can(role_int int, action AuthAction) bool {
	var role = AuthRole(role_int)
	return role.Can(action)
}

func Reset() {
	permissions = make(map[AuthRole]map[AuthAction]bool)
}
