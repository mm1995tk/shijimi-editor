package core

import ()

var (
	ctrlD = Key{Code: 'd', Mod: ModCtrl}
	ctrlU = Key{Code: 'u', Mod: ModCtrl}
)

var (
	ctrlAltD = Key{Code: 'd', Mod: ModCtrl | ModAlt}
	ctrlAltU = Key{Code: 'u', Mod: ModCtrl | ModAlt}
)
