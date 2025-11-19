package repository

type ExtDBpg struct {
	ID              int    `json:"ExtDBpg_id"`
	TableID         int    `json:"ExtDBpg_ExtDBtbl_id"`
	Title           string `json:"ExtDBpg_title"`
	Active          string `json:"active"`
	WebStatus       string `json:"ExtDBpg_webstatus"`
	WebStatusNew    string `json:"ExtDBpg_webstatusnew"`
	WebStatusFinish string `json:"ExtDBpg_webstatusdone"`
	AEdit           string `json:"ExtDBpg_aEdit"`
	ANew            string `json:"ExtDBpg_aNew"`
	Files           string `json:"ExtDBpg_files"`
	WebDate         string `json:"ExtDBpg_webdate"`
	WebBy           string `json:"ExtDBpg_webby"`
	Filter          string `json:"ExtDBpg_filter"`
	Order1          string `json:"ExtDBpg_order1"`
	Order2          string `json:"ExtDBpg_order2"`
	TblFormType     string `json:"ExtDBtbl_formtype"`
	TblType         string `json:"ExtDBtbl_type"`
	TblTitle        string `json:"ExtDBtbl_title"`
	CaPage          string `json:"ExtDBpg_capage"`
}
