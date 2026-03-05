package response

import "admin/internal/service"

type EnabledModule struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Endpoint  string `json:"endpoint"`
	Installed bool   `json:"installed"`
}

func NewEnabledModule(item service.EnabledModule) EnabledModule {
	return EnabledModule{
		Name:      item.Name,
		Status:    item.Status,
		Endpoint:  item.Endpoint,
		Installed: item.Installed,
	}
}
