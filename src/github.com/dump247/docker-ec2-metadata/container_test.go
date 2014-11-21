package main

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenerateSessionName(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("ner_path_tag-ender_colden-69efa4", generateSessionName(&docker.Container{
		ID:   "69efa4b5bea1c69ce4c30c0b81925f8e4299c5661e1a880feb6e0be5a9178f98",
		Name: "/tender_colden",
		Config: &docker.Config{
			Image: "docker-registry.host.com/container/path:tag",
		},
	}))

	assert.Equal("ner_path_tag-ender_colden-69efa4", generateSessionName(&docker.Container{
		ID:   "69efa4b5bea1c69ce4c30c0b81925f8e4299c5661e1a880feb6e0be5a9178f98",
		Name: "/ender_colden",
		Config: &docker.Config{
			Image: "docker-registry.host.com/container/path:tag",
		},
	}))

	assert.Equal("iner_path_tag-nder_colden-69efa4", generateSessionName(&docker.Container{
		ID:   "69efa4b5bea1c69ce4c30c0b81925f8e4299c5661e1a880feb6e0be5a9178f98",
		Name: "/nder_colden",
		Config: &docker.Config{
			Image: "docker-registry.host.com/container/path:tag",
		},
	}))

	assert.Equal("image-cont-69efa4", generateSessionName(&docker.Container{
		ID:   "69efa4b5bea1c69ce4c30c0b81925f8e4299c5661e1a880feb6e0be5a9178f98",
		Name: "/cont",
		Config: &docker.Config{
			Image: "image",
		},
	}))

	assert.Equal("image-container_name-69efa4", generateSessionName(&docker.Container{
		ID:   "69efa4b5bea1c69ce4c30c0b81925f8e4299c5661e1a880feb6e0be5a9178f98",
		Name: "/container_name",
		Config: &docker.Config{
			Image: "image",
		},
	}))
}
