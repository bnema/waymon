// +build cgo

package display

/*
#cgo pkg-config: wayland-client
#cgo CFLAGS: -I./generated
#include <wayland-client.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "generated/wlr-output-management-unstable-v1-client.h"

// Include the generated protocol code
#include "generated/wlr-output-management-unstable-v1-protocol.c"

typedef struct {
    char name[256];
    char description[256];
    int32_t x, y;
    int32_t width, height;
    int32_t phys_width, phys_height;
    int enabled;
    double scale;
    int32_t refresh; // mHz
} wlr_output_info;

typedef struct mode_data {
    struct zwlr_output_mode_v1 *mode;
    int32_t width, height;
    int32_t refresh;
    struct mode_data *next;
} mode_data;

typedef struct {
    struct zwlr_output_head_v1 *head;
    struct zwlr_output_mode_v1 *current_mode;
    mode_data *modes;
    wlr_output_info info;
    int index;
} wlr_output_data;

static struct wl_display *display = NULL;
static struct wl_registry *registry = NULL;
static struct zwlr_output_manager_v1 *output_manager = NULL;
static wlr_output_data wlr_outputs[32];
static int wlr_output_count = 0;
static int wlr_done = 0;

// Forward declaration
static const struct zwlr_output_mode_v1_listener mode_listener;

// Head event handlers
static void head_name(void *data, struct zwlr_output_head_v1 *head, const char *name) {
    wlr_output_data *out = (wlr_output_data *)data;
    if (name) {
        strncpy(out->info.name, name, 255);
        out->info.name[255] = '\0';
    }
}

static void head_description(void *data, struct zwlr_output_head_v1 *head, const char *description) {
    wlr_output_data *out = (wlr_output_data *)data;
    if (description) {
        strncpy(out->info.description, description, 255);
        out->info.description[255] = '\0';
    }
}

static void head_physical_size(void *data, struct zwlr_output_head_v1 *head, int32_t width, int32_t height) {
    wlr_output_data *out = (wlr_output_data *)data;
    out->info.phys_width = width;
    out->info.phys_height = height;
}

static void head_mode(void *data, struct zwlr_output_head_v1 *head, struct zwlr_output_mode_v1 *mode) {
    wlr_output_data *out = (wlr_output_data *)data;
    // fprintf(stderr, "DEBUG: head_mode called for output %d, mode=%p\n", out->index, mode);
    
    // Create a mode entry and add to list
    mode_data *md = malloc(sizeof(mode_data));
    md->mode = mode;
    md->width = 0;
    md->height = 0;
    md->refresh = 0;
    md->next = out->modes;
    out->modes = md;
    
    // Add listener to get mode details
    zwlr_output_mode_v1_add_listener(mode, &mode_listener, md);
}

static void head_enabled(void *data, struct zwlr_output_head_v1 *head, int32_t enabled) {
    wlr_output_data *out = (wlr_output_data *)data;
    out->info.enabled = enabled;
}

static void head_current_mode(void *data, struct zwlr_output_head_v1 *head, struct zwlr_output_mode_v1 *mode) {
    wlr_output_data *out = (wlr_output_data *)data;
    // fprintf(stderr, "DEBUG: head_current_mode called for output %d, mode=%p\n", out->index, mode);
    out->current_mode = mode;
}

static void head_position(void *data, struct zwlr_output_head_v1 *head, int32_t x, int32_t y) {
    wlr_output_data *out = (wlr_output_data *)data;
    out->info.x = x;
    out->info.y = y;
}

static void head_transform(void *data, struct zwlr_output_head_v1 *head, int32_t transform) {
    // We don't need transform for now
}

static void head_scale(void *data, struct zwlr_output_head_v1 *head, wl_fixed_t scale) {
    wlr_output_data *out = (wlr_output_data *)data;
    out->info.scale = wl_fixed_to_double(scale);
}

static void head_finished(void *data, struct zwlr_output_head_v1 *head) {
    // Head is no longer valid
}

static void head_make(void *data, struct zwlr_output_head_v1 *head, const char *make) {
    // We don't need manufacturer info for now
}

static void head_model(void *data, struct zwlr_output_head_v1 *head, const char *model) {
    // Model is often included in description
}

static void head_serial_number(void *data, struct zwlr_output_head_v1 *head, const char *serial_number) {
    // We don't need serial number for now
}

static void head_adaptive_sync(void *data, struct zwlr_output_head_v1 *head, uint32_t state) {
    // We don't need adaptive sync info for now
}

static const struct zwlr_output_head_v1_listener head_listener = {
    .name = head_name,
    .description = head_description,
    .physical_size = head_physical_size,
    .mode = head_mode,
    .enabled = head_enabled,
    .current_mode = head_current_mode,
    .position = head_position,
    .transform = head_transform,
    .scale = head_scale,
    .finished = head_finished,
    .make = head_make,
    .model = head_model,
    .serial_number = head_serial_number,
    .adaptive_sync = head_adaptive_sync,
};

// Mode event handlers
static void mode_size(void *data, struct zwlr_output_mode_v1 *mode, int32_t width, int32_t height) {
    mode_data *md = (mode_data *)data;
    md->width = width;
    md->height = height;
    // fprintf(stderr, "DEBUG: Mode size: %dx%d\n", width, height);
}

static void mode_refresh(void *data, struct zwlr_output_mode_v1 *mode, int32_t refresh) {
    mode_data *md = (mode_data *)data;
    md->refresh = refresh;
}

static void mode_preferred(void *data, struct zwlr_output_mode_v1 *mode) {
    // We don't need to track preferred mode for now
}

static void mode_finished(void *data, struct zwlr_output_mode_v1 *mode) {
    // Mode is no longer valid
}

static const struct zwlr_output_mode_v1_listener mode_listener = {
    .size = mode_size,
    .refresh = mode_refresh,
    .preferred = mode_preferred,
    .finished = mode_finished,
};

// Manager event handlers
static void manager_head(void *data, struct zwlr_output_manager_v1 *manager, struct zwlr_output_head_v1 *head) {
    if (wlr_output_count < 32) {
        wlr_outputs[wlr_output_count].head = head;
        wlr_outputs[wlr_output_count].index = wlr_output_count;
        wlr_outputs[wlr_output_count].info.scale = 1.0; // Default
        
        zwlr_output_head_v1_add_listener(head, &head_listener, &wlr_outputs[wlr_output_count]);
        
        wlr_output_count++;
    }
}

static void manager_done(void *data, struct zwlr_output_manager_v1 *manager, uint32_t serial) {
    wlr_done = 1;
}

static void manager_finished(void *data, struct zwlr_output_manager_v1 *manager) {
    // Manager is no longer valid
}

static const struct zwlr_output_manager_v1_listener manager_listener = {
    .head = manager_head,
    .done = manager_done,
    .finished = manager_finished,
};

// Registry handler
static void registry_handler(void *data, struct wl_registry *registry, uint32_t id, const char *interface, uint32_t version) {
    if (strcmp(interface, zwlr_output_manager_v1_interface.name) == 0) {
        output_manager = wl_registry_bind(registry, id, &zwlr_output_manager_v1_interface, 1);
        zwlr_output_manager_v1_add_listener(output_manager, &manager_listener, NULL);
    }
}

static void registry_remover(void *data, struct wl_registry *registry, uint32_t id) {
}

static const struct wl_registry_listener registry_listener = {
    .global = registry_handler,
    .global_remove = registry_remover,
};

// Mode handler for current mode
static void handle_current_mode(wlr_output_data *out, struct zwlr_output_mode_v1 *mode) {
    if (mode) {
        zwlr_output_mode_v1_add_listener(mode, &mode_listener, out);
    }
}

// Get display info using wlr-output-management protocol
int get_wlr_outputs(wlr_output_info **out_outputs) {
    display = wl_display_connect(NULL);
    if (!display) {
        return -1;
    }
    
    wlr_output_count = 0;
    wlr_done = 0;
    memset(wlr_outputs, 0, sizeof(wlr_outputs));
    
    registry = wl_display_get_registry(display);
    wl_registry_add_listener(registry, &registry_listener, NULL);
    
    wl_display_roundtrip(display);
    
    if (!output_manager) {
        wl_display_disconnect(display);
        return -2; // Protocol not supported
    }
    
    // Wait for initial events
    while (!wlr_done && wl_display_dispatch(display) != -1) {
        // Keep processing events
    }
    
    // Process all pending events to get mode information  
    wl_display_roundtrip(display);
    
    // Find the current mode data and copy its info
    for (int i = 0; i < wlr_output_count; i++) {
        if (wlr_outputs[i].current_mode) {
            // fprintf(stderr, "DEBUG: Looking for current mode for output %d\n", i);
            // Find the mode data that matches current_mode
            for (mode_data *md = wlr_outputs[i].modes; md; md = md->next) {
                if (md->mode == wlr_outputs[i].current_mode) {
                    wlr_outputs[i].info.width = md->width;
                    wlr_outputs[i].info.height = md->height;
                    wlr_outputs[i].info.refresh = md->refresh;
                    // fprintf(stderr, "DEBUG: Found current mode: %dx%d @ %dmHz\n", 
                    //         md->width, md->height, md->refresh);
                    break;
                }
            }
        }
    }
    
    // Copy output info
    static wlr_output_info static_wlr_outputs[32];
    for (int i = 0; i < wlr_output_count; i++) {
        static_wlr_outputs[i] = wlr_outputs[i].info;
    }
    *out_outputs = static_wlr_outputs;
    
    // Cleanup mode lists
    for (int i = 0; i < wlr_output_count; i++) {
        mode_data *md = wlr_outputs[i].modes;
        while (md) {
            mode_data *next = md->next;
            free(md);
            md = next;
        }
    }
    
    // Cleanup
    if (output_manager) {
        zwlr_output_manager_v1_destroy(output_manager);
    }
    wl_registry_destroy(registry);
    wl_display_disconnect(display);
    
    return wlr_output_count;
}
*/
import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

type wlrOutputManagementBackend struct{}

func newWlrOutputManagementBackend() (Backend, error) {
	// Check if running with sudo - this backend won't work with sudo
	if os.Getenv("SUDO_USER") != "" {
		return nil, fmt.Errorf("cannot use wlr-output-management backend with sudo")
	}
	
	return &wlrOutputManagementBackend{}, nil
}

func (w *wlrOutputManagementBackend) GetMonitors() ([]*Monitor, error) {
	var cOutputs *C.wlr_output_info
	count := C.get_wlr_outputs(&cOutputs)
	
	if count < 0 {
		if count == -1 {
			return nil, fmt.Errorf("failed to connect to Wayland display")
		} else if count == -2 {
			return nil, fmt.Errorf("wlr-output-management protocol not supported by compositor")
		}
		return nil, fmt.Errorf("failed to get outputs: error %d", count)
	}
	
	if count == 0 {
		return nil, fmt.Errorf("no outputs found")
	}
	
	// Convert C array to Go slice
	outputs := (*[32]C.wlr_output_info)(unsafe.Pointer(cOutputs))[:count:count]
	
	monitors := make([]*Monitor, 0, count)
	for i := 0; i < int(count); i++ {
		output := &outputs[i]
		
		// Only include enabled outputs
		if output.enabled == 0 {
			continue
		}
		
		name := C.GoString(&output.name[0])
		if name == "" {
			name = fmt.Sprintf("Unknown-%d", i)
		}
		
		description := C.GoString(&output.description[0])
		
		monitor := &Monitor{
			ID:      fmt.Sprintf("%d", i),
			Name:    name,
			X:       int32(output.x),
			Y:       int32(output.y),
			Width:   int32(output.width),
			Height:  int32(output.height),
			Scale:   float64(output.scale),
			Primary: i == 0, // First monitor is primary
		}
		
		// Use description as name if it's more descriptive
		if description != "" && len(description) > len(name) {
			monitor.Name = description
		}
		
		monitors = append(monitors, monitor)
	}
	
	return monitors, nil
}

func (w *wlrOutputManagementBackend) GetCursorPosition() (x, y int32, err error) {
	return 0, 0, fmt.Errorf("cursor position not available via wlr-output-management")
}

func (w *wlrOutputManagementBackend) Close() error {
	return nil
}