//go:build cgo
// +build cgo

package display

/*
#cgo pkg-config: wayland-client
#include <wayland-client.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Simple output info structure
typedef struct {
    char name[256];
    int x, y;
    int width, height;
    int scale;
    int enabled;
} output_info;

static output_info outputs[32];
static int output_count = 0;

// wl_output listener callbacks
static void output_geometry(void *data, struct wl_output *output,
    int x, int y, int physical_width, int physical_height,
    int subpixel, const char *make, const char *model, int transform) {

    int index = (intptr_t)data;
    if (index < 32) {
        outputs[index].x = x;
        outputs[index].y = y;
        if (model && strlen(model) > 0) {
            strncpy(outputs[index].name, model, 255);
        }
    }
}

static void output_mode(void *data, struct wl_output *output,
    uint32_t flags, int width, int height, int refresh) {

    int index = (intptr_t)data;
    // WL_OUTPUT_MODE_CURRENT = 0x1
    if (index < 32 && (flags & 0x1)) {
        outputs[index].width = width;
        outputs[index].height = height;
        outputs[index].enabled = 1;
    }
}

static void output_done(void *data, struct wl_output *output) {}
static void output_scale(void *data, struct wl_output *output, int32_t scale) {
    int index = (intptr_t)data;
    if (index < 32) {
        outputs[index].scale = scale;
    }
}

static const struct wl_output_listener output_listener = {
    .geometry = output_geometry,
    .mode = output_mode,
    .done = output_done,
    .scale = output_scale,
};

// Registry handler
static void registry_handler(void *data, struct wl_registry *registry,
    uint32_t id, const char *interface, uint32_t version) {

    if (strcmp(interface, "wl_output") == 0 && output_count < 32) {
        struct wl_output *output = wl_registry_bind(registry, id, &wl_output_interface, 2);
        wl_output_add_listener(output, &output_listener, (void*)(intptr_t)output_count);

        // Set default name
        snprintf(outputs[output_count].name, 255, "output-%d", output_count);
        outputs[output_count].scale = 1;
        output_count++;
    }
}

static void registry_remover(void *data, struct wl_registry *registry, uint32_t id) {}

static const struct wl_registry_listener registry_listener = {
    .global = registry_handler,
    .global_remove = registry_remover,
};

// Get display info using Wayland
int get_wayland_outputs(output_info **out_outputs) {
    struct wl_display *display = wl_display_connect(NULL);
    if (!display) {
        return -1;
    }

    output_count = 0;
    memset(outputs, 0, sizeof(outputs));

    struct wl_registry *registry = wl_display_get_registry(display);
    wl_registry_add_listener(registry, &registry_listener, NULL);

    // Initial roundtrip to get registry events
    wl_display_roundtrip(display);
    // Second roundtrip to get output events
    wl_display_roundtrip(display);

    wl_registry_destroy(registry);
    wl_display_disconnect(display);

    *out_outputs = outputs;
    return output_count;
}
*/
import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

// wlrCgoBackend uses CGO to directly call Wayland client library
type wlrCgoBackend struct{}

func newWlrCgoBackend() (Backend, error) {
	// CGO backend doesn't work with sudo because it can't connect to the user's Wayland session
	// Just fail silently and let it fall back to wlr-randr which handles sudo properly
	if os.Getenv("SUDO_USER") != "" {
		return nil, fmt.Errorf("CGO backend not available under sudo")
	}
	return &wlrCgoBackend{}, nil
}

func (w *wlrCgoBackend) GetMonitors() ([]*Monitor, error) {
	var cOutputs *C.output_info
	count := int(C.get_wayland_outputs(&cOutputs))

	if count < 0 {
		return nil, fmt.Errorf("failed to connect to Wayland display")
	}

	if count == 0 {
		return nil, fmt.Errorf("no outputs found")
	}

	// Convert C array to Go slice
	outputs := (*[32]C.output_info)(unsafe.Pointer(cOutputs))[:count:count]

	monitors := make([]*Monitor, 0, count)
	for i, out := range outputs {
		if out.enabled == 0 || out.width == 0 || out.height == 0 {
			continue
		}

		monitor := &Monitor{
			ID:      fmt.Sprintf("%d", i),
			Name:    C.GoString(&out.name[0]),
			X:       int32(out.x),
			Y:       int32(out.y),
			Width:   int32(out.width),
			Height:  int32(out.height),
			Scale:   float64(out.scale),
			Primary: i == 0 || (out.x == 0 && out.y == 0),
		}
		monitors = append(monitors, monitor)
	}

	if len(monitors) == 0 {
		return nil, fmt.Errorf("no enabled monitors found")
	}

	return monitors, nil
}

func (w *wlrCgoBackend) GetCursorPosition() (x, y int32, err error) {
	return 0, 0, fmt.Errorf("cursor position not available via Wayland")
}

func (w *wlrCgoBackend) Close() error {
	return nil
}
