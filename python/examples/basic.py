"""Run with: python basic.py model.ocrpack image.png"""
import sys

from oneocr_native import OneOcrEngine

with OneOcrEngine(sys.argv[1]) as engine:
    print(engine.recognize(sys.argv[2]).text)
