// Star Catch - a tiny Arduboy game used as a porting exercise.
// Copyright (c) 2026 BeautifulTinyGo contributors. MIT License.
//
// Move the basket with LEFT/RIGHT, catch falling stars. A star caught = +1,
// a star missed = one life lost. A starts a game, B toggles sound.
// The best score is kept in EEPROM.

#include <Arduboy2.h>
#include <ArduboyTones.h>
#include <EEPROM.h>

Arduboy2 arduboy;
Sprites sprites;
ArduboyTones sound(arduboy.audio.enabled);

// 16x8 basket, Sprites format: width, height, frame data (one frame).
const unsigned char PROGMEM basket[] = {
  16, 8,
  0x03, 0x07, 0x0d, 0x19, 0x31, 0x61, 0xc1, 0x81,
  0x81, 0xc1, 0x61, 0x31, 0x19, 0x0d, 0x07, 0x03,
};

// 8x8 star, two animation frames, Sprites format.
const unsigned char PROGMEM star[] = {
  8, 8,
  0x08, 0x18, 0x3c, 0xff, 0x3c, 0x18, 0x08, 0x00,
  0x10, 0x18, 0x7c, 0x3f, 0x7c, 0x18, 0x10, 0x00,
};

// 8x8 heart for the lives display, drawBitmap format (no header).
const unsigned char PROGMEM heart[] = {
  0x0c, 0x1e, 0x3e, 0x7c, 0x3e, 0x1e, 0x0c, 0x00,
};

const uint16_t catchSound[] PROGMEM = { 1047, 40, 1319, 60, TONES_END };
const uint16_t missSound[] PROGMEM = { 196, 120, TONES_END };

const int EEPROM_ADDR = EEPROM_STORAGE_SPACE_START + 40;
const uint8_t MAX_STARS = 5;

struct Star {
  int16_t x;
  int16_t y;      // in 1/4 pixels
  uint8_t speed;  // 1/4 pixels per frame
  bool live;
};

enum State : uint8_t { TITLE, PLAY, OVER };

State state = TITLE;
Star stars[MAX_STARS];
int16_t basketX = 56;
uint16_t score = 0;
uint16_t best = 0;
uint8_t lives = 3;

void setup() {
  arduboy.begin();
  arduboy.setFrameRate(60);
  arduboy.initRandomSeed();
  EEPROM.get(EEPROM_ADDR, best);
  if (best == 0xFFFF) best = 0;
}

void resetGame() {
  for (uint8_t i = 0; i < MAX_STARS; i++) stars[i].live = false;
  basketX = 56;
  score = 0;
  lives = 3;
  state = PLAY;
}

void spawnStar() {
  for (uint8_t i = 0; i < MAX_STARS; i++) {
    if (!stars[i].live) {
      stars[i].x = random(0, WIDTH - 8);
      stars[i].y = 8 * 4;
      stars[i].speed = 3 + min(score / 4, 6);
      stars[i].live = true;
      return;
    }
  }
}

void updatePlay() {
  if (arduboy.pressed(LEFT_BUTTON) && basketX > 0) basketX -= 2;
  if (arduboy.pressed(RIGHT_BUTTON) && basketX < WIDTH - 16) basketX += 2;
  if (arduboy.everyXFrames(40)) spawnStar();

  Rect basketRect = { basketX, HEIGHT - 8, 16, 8 };
  for (uint8_t i = 0; i < MAX_STARS; i++) {
    Star &s = stars[i];
    if (!s.live) continue;
    s.y += s.speed;
    Rect starRect = { s.x, s.y / 4, 8, 8 };
    if (arduboy.collide(starRect, basketRect)) {
      s.live = false;
      score++;
      sound.tones(catchSound);
    } else if (s.y / 4 >= HEIGHT) {
      s.live = false;
      sound.tones(missSound);
      if (--lives == 0) {
        state = OVER;
        if (score > best) {
          best = score;
          EEPROM.put(EEPROM_ADDR, best);
        }
      }
    }
  }
}

void drawPlay() {
  for (uint8_t i = 0; i < MAX_STARS; i++) {
    if (stars[i].live) {
      uint8_t frame = (arduboy.frameCount / 8) % 2;
      sprites.drawSelfMasked(stars[i].x, stars[i].y / 4, star, frame);
    }
  }
  sprites.drawOverwrite(basketX, HEIGHT - 8, basket, 0);
  for (uint8_t i = 0; i < lives; i++) {
    arduboy.drawBitmap(WIDTH - 9 * (i + 1), 0, heart, 8, 8, WHITE);
  }
  arduboy.setCursor(0, 0);
  arduboy.print(score);
  arduboy.drawFastHLine(0, 8, WIDTH, WHITE);
}

void loop() {
  if (!arduboy.nextFrame()) return;
  arduboy.pollButtons();
  if (arduboy.justPressed(B_BUTTON)) {
    if (arduboy.audio.enabled()) arduboy.audio.off(); else arduboy.audio.on();
  }
  arduboy.clear();

  switch (state) {
    case TITLE:
      arduboy.setCursor(34, 20);
      arduboy.print(F("STAR CATCH"));
      arduboy.setCursor(28, 40);
      arduboy.print(F("A: START  B: SND"));
      if (arduboy.justPressed(A_BUTTON)) resetGame();
      break;
    case PLAY:
      updatePlay();
      drawPlay();
      break;
    case OVER:
      arduboy.setCursor(37, 16);
      arduboy.print(F("GAME OVER"));
      arduboy.setCursor(37, 30);
      arduboy.print(F("SCORE "));
      arduboy.print(score);
      arduboy.setCursor(37, 40);
      arduboy.print(F("BEST  "));
      arduboy.print(best);
      if (arduboy.justPressed(A_BUTTON)) resetGame();
      break;
  }
  arduboy.display();
}
