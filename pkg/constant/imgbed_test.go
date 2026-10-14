package constant

import "testing"

func TestImgBedStorageTypeIsSupported(t *testing.T) {
	if !StoragePolicyType("imgbed").IsValid() {
		t.Fatal("ImgBed must be accepted as a storage policy type")
	}
}
