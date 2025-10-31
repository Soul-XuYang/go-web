package config

const (
	default_filepath = "./files"
)

func initPath() {
	if AppConfig.Upload.Storagepath == "" {
		AppConfig.Upload.Storagepath = default_filepath
	}
}


