package config

type BE struct {
	MLURL string `json:"mlurl" yaml:"mlurl"`
}

func (b *BE)Bind()*BE{
	b.MLURL = GetActualValue(b.MLURL)
	return b
}
