// Package request contains HTTP request DTOs.
package request

// CreateProject represents project creation request
type CreateProject struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Template        string   `json:"template"`
	AppPaths        []string `json:"app_paths"`
	FilesPaths      []string `json:"files_paths"`
	BlacklistExts   []string `json:"blacklist_exts"`
	WhitelistPaths  []string `json:"whitelist_paths"`
	DBHost         string   `json:"db_host"`
	DBUser         string   `json:"db_user"`
	DBPass         string   `json:"db_pass"`
	DBName         string   `json:"db_name"`
}

// UpdateProject represents project update request
type UpdateProject struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Template        string   `json:"template"`
	AppPaths        []string `json:"app_paths"`
	FilesPaths      []string `json:"files_paths"`
	BlacklistExts   []string `json:"blacklist_exts"`
	WhitelistPaths  []string `json:"whitelist_paths"`
	DBHost         string   `json:"db_host"`
	DBUser         string   `json:"db_user"`
	DBPass         string   `json:"db_pass"`
	DBName         string   `json:"db_name"`
}

// Login represents login request
type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
