package api

type IDName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewIDName(id, name string) IDName {
	return IDName{
		ID:   id,
		Name: name,
	}
}

type Cluster struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Site     IDName   `json:"site"`
	Zone     string   `json:"zone"`
	Images   []string `json:"image_type"`
	Networks []IDName `json:"network"`
	HATag    string   `json:"ha_tag"`
	Desc     string   `json:"desc"`
	Enabled  bool     `json:"enabled"`
	Created  Editor   `json:"created"`
	Modified Editor   `json:"modified"`
}

type ClusterConfig struct {
	Name    string   `json:"name"`
	Site    string   `json:"site_id"`
	Zone    string   `json:"zone"`
	Images  []string `json:"image_type,omitempty"`
	HATag   string   `json:"ha_tag"`
	Desc    string   `json:"desc"`
	User    string   `json:"created_user"`
	Enabled bool     `json:"enabled"`
}

func (cc ClusterConfig) Valid() error {
	return nil
}

type ClusterOptions struct {
	Name    *string  `json:"name,omitempty"`
	Zone    *string  `json:"zone,omitempty"`
	Images  []string `json:"Images,omitempty"`
	HaTag   *string  `json:"ha_tag,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
	Desc    *string  `json:"desc,omitempty"`
	User    string   `json:"modified_user"`
}

type ClustersResponse []Cluster
