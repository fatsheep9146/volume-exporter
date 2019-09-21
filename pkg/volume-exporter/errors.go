package controller

import (
	"errors"
)

var (
	PVCNotFound        = errors.New("PVCNotFound")        // the pvc that pod uses is not found
	MountPointNotReady = errors.New("MountPointNotReady") // the mount point of pvc is not created on the host

)
