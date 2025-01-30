package template

type OnlyId struct {
	Id int64 `json:"id" db:"id"`
}

type KeyAndValue struct {
	Key   string `json:"key" db:"key"`
	Value string `json:"value" db:"value"`
}
