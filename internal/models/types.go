package models

// Domain Metadata
type DomMetadata struct {
	Result struct {
		Presigned []string `json:"PRESIGNED"`
	} `json:"result"`
}

// Start of Authority
type Soa struct {
	Qtype    string `json:"qtype"`
	Qname    string `json:"qname"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	DomainID int    `json:"domain_id"`
}

// Container for SOA Record
type SoaResult struct {
	Result []Soa `json:"result"`
}

// A Record
type ARecord struct {
	Qtype   string `json:"qtype"`
	Qname   string `json:"qname"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

// A Record container
type AResult struct {
	Result []ARecord `json:"result"`
	Log    []string  `json:"log"`
}

// Application Instance
type Instance struct {
	Ip          string `yaml:"host" redis:"ip"`
	Healthy     string `redis:"healthy"` //Either "yes" or "no"
	Application string `redis:"application"`
	Count       int64  `redis:"count"`
}

// Application
type Application struct {
	Name        string `yaml:"name"`
	Method      string `yaml:"method"`
	Hostname    string `yaml:"hostname"`
	Healthcheck struct {
		Type string `yaml:"type"`
		Path string `yaml:"path"`
		Port int    `yaml:"port"`
	} `yaml:"healthcheck,omitempty"`
	Instances []Instance `yaml:"instances"`
}

// Network Region
type Region struct {
	Region  string   `yaml:"region"`
	Subnets []string `yaml:"subnets"`
	Prefer  string   `yaml:"prefer,omitempty"`
}

// Sextant Configuration
type Configuration struct {
	Sextant struct {
		Verbose      bool   `yaml:"verbose"`
		APIPort      int    `yaml:"api_port"`
		Fqdn         string `yaml:"fqdn"`
		Healthchecks struct {
			Frequency int `yaml:"frequency"`
		} `yaml:"healthchecks"`
		Redis struct {
			Host     string `yaml:"host"`
			Port     string `yaml:"port"`
			Password string `yaml:"password"`
			Protocol string `yaml:"protocol"`
			Database string `yaml:"database"`
		} `yaml:"redis"`
		Soa struct {
			Email      string `yaml:"email"`
			Serial     string `yaml:"serial"`
			Refresh    string `yaml:"refresh"`
			Retry      string `yaml:"retry"`
			Expiration string `yaml:"expiration"`
			TTL        string `yaml:"ttl"`
		} `yaml:"soa"`
	} `yaml:"sextant"`
	Applications []Application `yaml:"applications"`
	Regions      []Region      `yaml:"regions"`
}
