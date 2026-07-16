//go:build windows

package main

import "golang.org/x/sys/windows"

func listStorageVolumes() []storageVolume {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	volumes := make([]storageVolume, 0, 4)
	for i := uint(0); i < 26; i++ {
		if mask&(1<<i) != 0 {
			volumes = append(volumes, storageVolume{Path: string(rune('A'+i)) + `:\`})
		}
	}
	return volumes
}
