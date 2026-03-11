package aws

import (
	"testing"

	"samebits.com/evidra/internal/canon"
)

func TestSecurityGroupOpen(t *testing.T) {
	t.Parallel()
	d := &SecurityGroupOpen{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`{
  "resource_changes": [{
    "type": "aws_security_group",
    "name": "web",
    "change": {
      "actions": ["create"],
      "after": {
        "ingress": [{
          "from_port": 22,
          "to_port": 22,
          "cidr_blocks": ["0.0.0.0/0"]
        }]
      }
    }
  }]
}`)) {
		t.Fatalf("expected security_group_open detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`{
  "resource_changes": [{
    "type": "aws_security_group",
    "name": "web",
    "change": {
      "actions": ["create"],
      "after": {
        "ingress": [{
          "from_port": 22,
          "to_port": 22,
          "cidr_blocks": ["10.0.0.0/8"]
        }]
      }
    }
  }]
}`)) {
		t.Fatalf("did not expect security_group_open detection")
	}
}

func TestRDSPublic(t *testing.T) {
	t.Parallel()
	d := &RDSPublic{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`{
  "resource_changes": [{
    "type": "aws_db_instance",
    "name": "db",
    "change": {"actions": ["create"], "after": {"publicly_accessible": true}}
  }]
}`)) {
		t.Fatalf("expected rds_public detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`{
  "resource_changes": [{
    "type": "aws_db_instance",
    "name": "db",
    "change": {"actions": ["create"], "after": {"publicly_accessible": false}}
  }]
}`)) {
		t.Fatalf("did not expect rds_public detection")
	}
}

func TestEBSUnencrypted(t *testing.T) {
	t.Parallel()
	d := &EBSUnencrypted{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`{
  "resource_changes": [{
    "type": "aws_ebs_volume",
    "name": "vol",
    "change": {"actions": ["create"], "after": {"encrypted": false}}
  }]
}`)) {
		t.Fatalf("expected ebs_unencrypted detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`{
  "resource_changes": [{
    "type": "aws_ebs_volume",
    "name": "vol",
    "change": {"actions": ["create"], "after": {"encrypted": true}}
  }]
}`)) {
		t.Fatalf("did not expect ebs_unencrypted detection")
	}
}
