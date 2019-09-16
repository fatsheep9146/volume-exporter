

# if ARCH, OS is not defined, then use `go env` to get the value
ARCH ?= $(shell go env GOARCH)
OS ?= $(shell go env GOOS)

# info about the image registry
REGISTRY ?= reg.kpaas.io/kpaas
TAG ?= latest 

OUT_DIR := _output/bin
CMD_DIR := cmd
BUILD_DIR := build

# all target binaries to be built
TARGETS := volume-exporter

# all pattern rules for all targets
# build_app1
# build_app1_linux-amd64
# image_app1
# push_app1
TARGETS_WITH_BUILD := $(addprefix build_,$(TARGETS))
TARGETS_WITH_OS_ARCH := $(addsuffix _%,$(addprefix build_,$(TARGETS)))
TARGETS_WITH_IMAGE := $(addprefix image_,$(TARGETS))
TARGETS_WITH_PUSH := $(addprefix push_,$(TARGETS))

$(TARGETS):
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(OUT_DIR)/linux/$@ ./$(CMD_DIR)/$@

$(TARGETS_WITH_BUILD):
	@$(MAKE) OS=$(OS) ARCH=$(ARCH) $(lastword $(subst _, ,$@))

$(TARGETS_WITH_OS_ARCH):
	@$(MAKE)							\
	OS=$(firstword $(subst -, ,$*))		\
	ARCH=$(lastword $(subst -, ,$*))	\
	$(word 2,$(subst _, ,$@))

$(TARGETS_WITH_IMAGE): image_%: build_%_linux-amd64
	cp $(OUT_DIR)/linux/$* $(BUILD_DIR)/$*/ 							
	docker build $(BUILD_DIR)/$* -t $(REGISTRY)/$*:$(TAG) -f $(BUILD_DIR)/$*/Dockerfile 
	rm -r $(BUILD_DIR)/$*/$*

$(TARGETS_WITH_PUSH): push_%: image_%
	docker push $(REGISTRY)/$*:$(TAG)
