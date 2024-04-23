package domain

type User struct {
	ID       int    `json:"id"    gorm:"type:serial;autoIncrement;primaryKey;unique;not null"`
	Login    string `json:"login" gorm:"type:string;size:256;unique;not null"`
	Password string `json:"password" gorm:"-:all"`
	Hash     string `gorm:"type:string;size:1000;not null"`
}

type Storage struct {
	ID      int    `json:"id"      gorm:"type:serial;autoIncrement;primaryKey;unique;not null"`
	Message string `json:"message" gorm:"type:string;not null"`
}
