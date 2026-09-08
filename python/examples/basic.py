"""Run with: python basic.py image.png"""
import argparse

from oneocr_native import OneOcrEngine

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("image")
args = parser.parse_args()
with OneOcrEngine() as engine:
    print(engine.recognize(args.image).text)
