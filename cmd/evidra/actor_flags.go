package main

import (
	"flag"

	"samebits.com/evidra/pkg/evidence"
)

type actorFlags struct {
	ID           string
	Type         string
	Origin       string
	InstanceID   string
	Version      string
	SkillVersion string
}

func bindActorFlags(fs *flag.FlagSet, opts *actorFlags, actorUsage string) {
	fs.StringVar(&opts.ID, "actor", "", actorUsage)
	fs.StringVar(&opts.Type, "actor-type", "", "Actor type (agent, cli, automation)")
	fs.StringVar(&opts.Origin, "actor-origin", "", "Actor origin/provenance (mcp-stdio, cli, automation)")
	fs.StringVar(&opts.InstanceID, "actor-instance-id", "", "Actor instance identifier")
	fs.StringVar(&opts.Version, "actor-version", "", "Actor software version")
	fs.StringVar(&opts.SkillVersion, "actor-skill-version", "", "Actor prompt/skill contract version")
}

func buildActor(opts actorFlags, defaultID, defaultType, defaultOrigin string) evidence.Actor {
	actorID := opts.ID
	if actorID == "" {
		actorID = defaultID
	}
	actorType := opts.Type
	if actorType == "" {
		actorType = defaultType
	}
	origin := opts.Origin
	if origin == "" {
		origin = defaultOrigin
	}
	return evidence.Actor{
		Type:         actorType,
		ID:           actorID,
		Provenance:   origin,
		InstanceID:   opts.InstanceID,
		Version:      opts.Version,
		SkillVersion: opts.SkillVersion,
	}
}
