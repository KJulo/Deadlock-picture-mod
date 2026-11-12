// Command catmod builds the soul container mod from a photo, like reference/scripts/catmod_cli.py.
//
//	go run ./cmd/catmod -downloads ../assets/cat.jpg     # VPK in build/go/ and a copy in ~/Downloads
//	go run ./cmd/catmod -install ../assets/cat.jpg       # straight into Deadlock's addons/pak09
//	go run ./cmd/catmod -layout wrap -back flip photo.jpg
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/build"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/photo"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/templates"
)

func hexColor(s string) ([3]uint8, error) {
	s = strings.TrimPrefix(s, "#")
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil || len(s) != 6 {
		return [3]uint8{}, fmt.Errorf("bad colour %q (want rrggbb)", s)
	}
	return [3]uint8{uint8(v >> 16), uint8(v >> 8), uint8(v)}, nil
}

func main() {
	home, _ := os.UserHomeDir()
	d := photo.DefaultParams()
	var (
		out        = flag.String("o", "../build/go/pak01_dir.vpk", "output VPK")
		tplDir     = flag.String("templates", "../templates", "game files dir (make fetch); $CATMOD_TEMPLATES overrides")
		variant    = flag.String("variant", d.Variant, "sphere | cage (original model, photo in the lens domes)")
		layout     = flag.String("layout", d.Layout, "cube6 | twice | wrap")
		cx         = flag.Float64("cx", d.Center[0], "photo point at the sticker centre, X (0..1)")
		cy         = flag.Float64("cy", d.Center[1], "photo point at the sticker centre, Y (0..1)")
		radius     = flag.Float64("radius", d.Radius, "photo disc radius (fraction of the shorter side)")
		projection = flag.String("projection", d.Projection, "equidistant | ortho")
		stickerDeg = flag.Float64("sticker-deg", d.StickerDeg, "sticker angular radius")
		ringDeg    = flag.Float64("ring-deg", d.RingDeg, "ring width (0 = none)")
		bg         = flag.String("bg-color", "18788c", "background colour (hex)")
		ring       = flag.String("ring-color", "ebebeb", "ring colour (hex)")
		back       = flag.String("back", d.Back, "wrap only: mirror | flip | color")
		alpha      = flag.Float64("color-alpha", d.ColorAlpha, "colour texture alpha (0 renders black)")
		rough      = flag.Float64("roughness", d.Roughness, "roughness 0..1")
		rotate     = flag.Float64("rotate", 0, "rotate the photo clockwise (degrees)")
		downloads  = flag.Bool("downloads", false, "also copy the VPK to ~/Downloads/catmod_go.vpk")
		install    = flag.Bool("install", false, "copy to Deadlock's addons/pak09_dir.vpk (restart the game)")
		lodTest    = flag.Bool("lodtest", false, "diagnostic: each LOD shows a coloured band labelled 'LOD n' (no photo needed)")
		citadel    = flag.String("citadel", filepath.Join(home, ".local/share/Steam/steamapps/common/Deadlock/game/citadel"), "Deadlock game/citadel dir")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: catmod [flags] PHOTO")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 && !*lodTest {
		flag.Usage()
		os.Exit(2)
	}
	fail := func(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }

	p := d
	p.Variant, p.Layout, p.Center, p.Radius, p.Projection = *variant, *layout, [2]float64{*cx, *cy}, *radius, *projection
	p.StickerDeg, p.RingDeg, p.Back = *stickerDeg, *ringDeg, *back
	p.ColorAlpha, p.Roughness, p.RotationDeg = *alpha, *rough, *rotate
	var err error
	if p.BgColor, err = hexColor(*bg); err != nil {
		fail(err)
	}
	if p.RingColor, err = hexColor(*ring); err != nil {
		fail(err)
	}

	start := time.Now()
	tpl, err := build.LoadTemplates(templates.Dir(*tplDir))
	if err != nil {
		fail(err)
	}
	var res *build.Result
	if *lodTest {
		res, err = build.LODTest(tpl)
	} else {
		var data []byte
		if data, err = os.ReadFile(flag.Arg(0)); err != nil {
			fail(err)
		}
		res, err = build.Mod(tpl, data, p)
	}
	if err != nil {
		fail(err)
	}
	pak, err := res.VPK()
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, pak, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s (%.1f MB) in %v\n", *out, float64(len(pak))/1e6, time.Since(start).Round(time.Millisecond))

	if *downloads {
		dst := filepath.Join(home, "Downloads", "catmod_go.vpk")
		if err := os.WriteFile(dst, pak, 0o644); err != nil {
			fail(err)
		}
		fmt.Println("copied to", dst)
	}
	if *install {
		// same files as the python CLI, so `catmod_cli.py uninstall` removes it
		addons := filepath.Join(*citadel, "addons")
		dst, marker := filepath.Join(addons, "pak09_dir.vpk"), filepath.Join(addons, ".catmod_pak09")
		if _, err := os.Stat(dst); err == nil {
			if _, err := os.Stat(marker); err != nil {
				fail(fmt.Errorf("%s exists and is not ours (DMM?), not overwriting", dst))
			}
		}
		if err := os.WriteFile(dst, pak, 0o644); err != nil {
			fail(err)
		}
		sum := sha256.Sum256(pak)
		if err := os.WriteFile(marker, []byte(hex.EncodeToString(sum[:])), 0o644); err != nil {
			fail(err)
		}
		fmt.Println("installed", dst, "(restart Deadlock; disable other soul container mods in DMM)")
	}
}
