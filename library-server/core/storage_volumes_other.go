//go:build !windows

package main

func listStorageVolumes() []storageVolume {
	return []storageVolume{{Path: "/"}}
}
