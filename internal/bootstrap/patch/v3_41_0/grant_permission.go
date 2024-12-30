package v3_41_0

import (
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
)

// GrantAdminPermissions gives admin Permission 0(can see hidden) - 9(webdav manage)
// This patch is written to help users upgrading from older version better adapt to PR AlistGo/alist#7705.
func GrantAdminPermissions() {
	admin, err := op.GetAdmin()
	if err != nil {
		utils.Log.Errorf("Cannot grant permissions to admin: %v", err)
	}
	if (admin.Permission & 0x3FF) == 0 {
		admin.Permission |= 0x3FF
	}
	err = op.UpdateUser(admin)
	if err != nil {
		utils.Log.Errorf("Cannot grant permissions to admin: %v", err)
	}
}
