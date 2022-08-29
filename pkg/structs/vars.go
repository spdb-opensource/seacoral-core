package structs

const (
	DefaultImagePullSecret = "regcred"
	OptionArch             = "arch"

	DefaultDataMount = "/DBAASDAT"
	DefaultLogMount  = "/DBAASLOG"
	DefaultBACKMount = "/DBAASBACKUP"

	NFSBackupTarget   = "/dbscale/backup/nfs"
	LocalBackupTarget = "/dbscale/backup/local"

	LocalBackupStorageType = "local"
	NFSBackupStorageType   = "nfs"
	S3BackupStorageType    = "s3"

	LabelGroup = "dbscale.app.group"
)
