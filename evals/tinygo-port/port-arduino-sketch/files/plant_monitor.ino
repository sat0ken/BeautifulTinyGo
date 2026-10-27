// Plant monitor - an Arduino Uno sketch used as a porting exercise.
// Copyright (c) 2026 BeautifulTinyGo contributors. MIT License.
//
// Hardware (Uno): soil sensor on A0, BH1750 light sensor on I2C (0x23),
// red LED on D9 (PWM, brightness shows how dry the soil is), button on D2
// to ground (INPUT_PULLUP) that cycles the dry threshold 30% -> 40% -> 50%.
// The threshold survives power cycles in EEPROM address 0.

#include <Wire.h>
#include <EEPROM.h>

const int SOIL_PIN = A0;
const int LED_PIN = 9;
const int BUTTON_PIN = 2;
const uint8_t BH1750_ADDR = 0x23;

// Calibration: raw ADC when the probe is in air (dry) and in water (wet).
const int RAW_DRY = 850;
const int RAW_WET = 380;

uint8_t threshold = 40;          // percent
unsigned long lastReport = 0;
unsigned long lastButtonChange = 0;
int lastButtonState = HIGH;
int stableButtonState = HIGH;

void setup() {
  Serial.begin(115200);
  pinMode(LED_PIN, OUTPUT);
  pinMode(BUTTON_PIN, INPUT_PULLUP);
  Wire.begin();
  Wire.beginTransmission(BH1750_ADDR);
  Wire.write(0x10);              // continuous high-resolution mode
  Wire.endTransmission();
  uint8_t saved = EEPROM.read(0);
  if (saved == 30 || saved == 40 || saved == 50) threshold = saved;
  Serial.println(F("plant monitor ready"));
}

int moisturePercent(int raw) {
  int pct = map(raw, RAW_DRY, RAW_WET, 0, 100);
  return constrain(pct, 0, 100);
}

long readLux() {
  Wire.requestFrom(BH1750_ADDR, (uint8_t)2);
  if (Wire.available() < 2) return -1;
  uint16_t v = (Wire.read() << 8) | Wire.read();
  return (long)(v / 1.2);
}

void handleButton() {
  int reading = digitalRead(BUTTON_PIN);
  if (reading != lastButtonState) {
    lastButtonChange = millis();
    lastButtonState = reading;
  }
  if (millis() - lastButtonChange > 30 && reading != stableButtonState) {
    stableButtonState = reading;
    if (stableButtonState == LOW) {
      threshold = threshold >= 50 ? 30 : threshold + 10;
      EEPROM.update(0, threshold);
      Serial.print(F("threshold="));
      Serial.println(threshold);
    }
  }
}

void loop() {
  handleButton();

  if (millis() - lastReport >= 2000) {
    lastReport = millis();
    int raw = analogRead(SOIL_PIN);
    int pct = moisturePercent(raw);
    long lux = readLux();

    // The drier the soil below the threshold, the brighter the LED.
    int brightness = 0;
    if (pct < threshold) {
      brightness = map(threshold - pct, 0, threshold, 40, 255);
    }
    analogWrite(LED_PIN, brightness);

    Serial.print(F("moisture="));
    Serial.print(pct);
    Serial.print(F("% raw="));
    Serial.print(raw);
    Serial.print(F(" lux="));
    if (lux < 0) Serial.print(F("err")); else Serial.print(lux);
    Serial.print(F(" led="));
    Serial.println(brightness);
  }
}
