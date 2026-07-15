package model

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/db"
	"time"
)

type ResourceHost struct {
	Id               int64           `json:"id"`
	Uid              string          `json:"uid"`
	Hash             string          `json:"hash"`
	Name             string          `json:"name"`
	PrivateIps       json.RawMessage `json:"private_ips"`
	Tags             json.RawMessage `json:"tags"`
	CloudProvider    string          `json:"cloud_provider"`
	ChargingMode     string          `json:"charging_mode"`
	Region           string          `json:"region"`
	AccountId        int64           `json:"account_id"`
	VpcId            string          `json:"vpc_id"`
	SubnetId         string          `json:"subnet_id"`
	SecurityGroups   json.RawMessage `json:"security_groups"`
	InstanceType     string          `json:"instance_type"`
	PublicIps        json.RawMessage `json:"public_ips"`
	AvailabilityZone string          `json:"availability_zone"`
	Status           string          `json:"status"`
	StreeGroup       string          `json:"stree_group"`
	StreeProduct     string          `json:"stree_product"`
	StreeApp         string          `json:"stree_app"`
	HostName         string          `json:"hostname" xorm:"-"`
	Sn               string          `json:"sn" xorm:"-"`
	Cpu              string          `json:"cpu" xorm:"cpu"`
	Mem              string          `json:"mem"`
	Disk             string          `json:"disk"`
	IpAddr           string          `json:"ip_addr" xorm:"-"`
	CreateTime       time.Time       `json:"create_time" xorm:"create_time created"`
	UpdateTime       time.Time       `json:"update_time" xorm:"update_time updated"`
}

type Collect struct {
	SN       string `json:"sn"`
	CPU      string `json:"cpu"`
	Mem      string `json:"mem"`
	Disk     string `json:"disk"`
	IpAddr   string `json:"ip_addr"`
	HostName string `json:"hostname"`
}

func (rh *ResourceHost) Add() error {
	_, err := db.Database["stree"].InsertOne(rh)
	return err
}

func (rh *ResourceHost) Update() (bool, error) {
	update, err := db.Database["stree"].Update(rh)
	if err != nil {
		return false, err
	}
	if update > 0 {
		return true, nil
	}
	return false, nil
}

func (rh *ResourceHost) UpdateByUID(uid string) (bool, error) {
	row, err := db.Database["stree"].Where("uid=?", uid).Update(rh)
	if err != nil {
		return false, err
	}

	if row > 0 {
		return true, nil
	}
	return false, nil
}

func (rh *ResourceHost) Get() (*ResourceHost, error) {
	get, err := db.Database["stree"].Get(rh)
	if err != nil {
		return nil, err
	}
	if !get {
		return nil, nil
	}
	return rh, nil
}

func (rh *ResourceHost) GenHash() string {
	h := md5.New()
	h.Write([]byte(rh.Sn))
	h.Write([]byte(rh.Name))
	h.Write([]byte(rh.IpAddr))
	h.Write([]byte(rh.Cpu))
	h.Write([]byte(rh.Mem))
	h.Write([]byte(rh.Disk))
	return hex.EncodeToString(h.Sum(nil))
}

func (rh *ResourceHost) Count() int64 {
	total, _ := db.Database["stree"].Where("id>0").Count(rh)
	return total
}

func GetResourceHostMultiple(where string, args ...interface{}) ([]ResourceHost, error) {
	var obj []ResourceHost
	err := db.Database["stree"].Where(where, args...).Find(&obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func BatchDeleteResource(tableName string, idKey string, ids []string) (int64, error) {
	raw := fmt.Sprintf("")
	res, err := db.Database["stree"].Exec(raw)
	if err != nil {
		return 0, err
	}
	row, err := res.RowsAffected()
	return row, err
}
