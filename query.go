package jqgrid

import (
	"encoding/json"
	"github.com/hwcer/cosgo/values"
	"github.com/hwcer/cosmo"
	"github.com/hwcer/cosmo/clause"
	"github.com/hwcer/cosmo/schema"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

var schemaOptions = schema.New()

type Query struct {
	sort         string
	order        string
	model        *schema.Schema
	filters      string
	searchOper   string
	searchField  string
	searchString string
}
type Filter struct {
	Rules   []*Rule `json:"rules"`
	GroupOp string  `json:"groupOp"`
}

type Rule struct {
	Op    string `json:"op"`
	Data  string `json:"data"`
	Field string `json:"field"`
}

func (this *Query) Bind(v url.Values) {
	if search := v.Get("_search"); search != "true" {
		return
	}
	this.sort = v.Get("sort")
	this.order = strings.ToUpper(v.Get("order"))
	this.filters = v.Get("filters")
	this.searchOper = v.Get("searchOper")
	this.searchField = v.Get("searchField")
	this.searchString = v.Get("searchString")
}

func (this *Query) Parse(q string) error {
	v, err := url.ParseQuery(q)
	if err != nil {
		return err
	}
	this.Bind(v)
	return nil
}

// Model 设置Schema *schema.Schema
func (this *Query) Model(model interface{}) (err error) {
	if v, ok := model.(*schema.Schema); ok {
		this.model = v
	} else {
		this.model, err = schema.Parse(model, schemaOptions)
	}

	return
}

func (this *Query) Page(db *cosmo.DB, body []byte) (*values.Paging, error) {
	q, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	this.Bind(q)
	page := q.Get("page")
	paging := &values.Paging{}
	paging.Page, _ = strconv.Atoi(page)
	size, _ := strconv.Atoi(q.Get("size"))
	paging.Init(size)

	tx := db
	if k, v := this.Order(); k != "" {
		tx = tx.Order(k, v)
	}
	filter, err := this.Filter()
	if err != nil {
		return nil, err
	}
	tx = tx.Where(filter).View(paging)
	return paging, tx.Error
}

func (this *Query) Order() (k string, v int) {
	if this.sort == "" || this.order == "" {
		return
	}
	k = this.sort
	if this.model != nil {
		field := this.model.LookUpField(this.sort)
		if field == nil {
			k = field.DBName
		}
	}
	if this.order == "DESC" {
		v = -1
	} else {
		v = 1
	}
	return
}

// Filter 编译查询语句，如果model不为空，可以格式化Field为dbname，val转换成数据库对应类型
func (this *Query) Filter() (clause.Filter, error) {
	var op string
	var args []interface{}
	var format []string

	query := clause.New()
	if this.filters != "" {
		i := &Filter{}
		if err := json.Unmarshal([]byte(this.filters), i); err != nil {
			return nil, err
		}
		oper := strings.Builder{}
		oper.WriteString(" ")
		oper.WriteString(strings.ToUpper(i.GroupOp))
		oper.WriteString(" ")
		op = oper.String()
		for _, r := range i.Rules {
			args = append(args, this.value(r.Field, r.Data))
			format = append(format, this.format(r.Field, r.Op))
		}

	} else if this.searchField != "" && this.searchString != "" {
		op = " AND "
		args = append(args, this.value(this.searchField, this.searchString))
		format = append(format, this.format(this.searchField, this.searchOper))
	}

	if len(format) == 0 {
		return nil, nil
	}
	query.Where(strings.Join(format, op), args...)
	return query.Build(this.model), nil
}

func (this *Query) value(k, v string) (r interface{}) {
	if this.model == nil {
		return v
	}
	field := this.model.LookUpField(k)
	if field == nil {
		return v
	}
	switch field.FieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		r, _ = strconv.Atoi(v)
	case reflect.Float32, reflect.Float64:
		r, _ = strconv.ParseFloat(v, 64)
	default:
		r = v
	}
	return
}

func (this *Query) format(k, op string) string {
	arr := []string{k, "", "?"}
	switch op {
	case "ne":
		arr[1] = "<>"
	case "lt":
		arr[1] = "<"
	case "le":
		arr[1] = "<="
	case "gt":
		arr[1] = ">"
	case "ge":
		arr[1] = ">="
	case "co":
		arr[1] = "IN"
	default:
		arr[1] = "="
	}
	return strings.Join(arr, " ")
}
