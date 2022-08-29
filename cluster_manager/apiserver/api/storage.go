package api

type RemoteStoragesResponse []RemoteStorage

type RemoteStorage struct {
	Enabled bool   `json:"enabled"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Desc    string `json:"desc"`
	Site    IDName `json:"site"`
	// 主机注册类型
	// enum: FC,iSCSI
	Type   string `json:"type"`
	Model  string `json:"model"`
	Vendor string `json:"vendor"`
	Auth   Auth   `json:"auth"`
	// enum: passing,critical,unknown
	Status string `json:"status"`
	ResourceStatus
	Task     TaskBrief `json:"task"`
	Created  Editor    `json:"created"`
	Modified Editor    `json:"modified"`
}

type RemoteStorageConfig struct {
	Name    string `json:"name"`
	Site    string `json:"site_id"`
	Desc    string `json:"desc"`
	Type    string `json:"type"`
	Model   string `json:"model"`
	Vendor  string `json:"vendor"`
	Version string `json:"API_version"`

	Auth Auth `json:"auth"`

	Enabled bool   `json:"enabled"`
	User    string `json:"created_user"`
}

type Auth struct {
	Port       int    `json:"port"`
	IP         IP     `json:"ip"`
	User       string `json:"username"`
	Password   string `json:"password"`
	Vstorename string `json:"vstorename,omitempty"`
}

func (c RemoteStorageConfig) Valid() error {

	return nil
}

type RemoteStorageOptions struct {
	Name *string `json:"name,omitempty"`
	Desc *string `json:"desc,omitempty"`
	User string  `json:"modified_user"`
	Auth struct {
		Port       *int    `json:"port,omitempty"`
		IP         *string `json:"ip,omitempty"`
		User       *string `json:"username,omitempty"`
		Password   *string `json:"password,omitempty"`
		Vstorename *string `json:"vstorename,omitempty"`
	} `json:"auth,omitempty"`

	Enabled *bool `json:"enabled,omitempty"`
}

type RemoteStoragePoolConfig struct {
	Enabled     bool        `json:"enabled"`
	Name        string      `json:"name"`
	Storage     string      `json:"storage_id"`
	Native      string      `json:"native_id"`
	Performance Performance `json:"performance"`
	Desc        string      `json:"desc"`
	User        string      `json:"created_user"`
}

func (rc RemoteStoragePoolConfig) Valid() error {
	return nil
}

type RemoteStoragePool struct {
	Enabled     bool        `json:"enabled"`
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Storage     IDName      `json:"storage"`
	Native      string      `json:"native_id"`
	Desc        string      `json:"desc"`
	Performance Performance `json:"performance"`
	ResourceStatus
	Task     TaskBrief `json:"task"`
	Created  Editor    `json:"created"`
	Modified Editor    `json:"modified"`
}

type RemoteStoragePoolsResponse []RemoteStoragePool

type RemoteStoragePoolOptions struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Name    *string `json:"name,omitempty"`
	Desc    *string `json:"desc,omitempty"`

	User string `json:"modified_user"`
}
