//go:build gopher_board_spi

package game

// W on gopher-board (ST7789 240x240): koebiten 0.6.0 shows a 100x64 screen
// there, scaled 2x, and ignores pixels at x >= 100.
const W = 100
