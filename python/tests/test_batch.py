from oneocr_native.engine import OneOcrEngine, RecognitionBatchItem


def test_batch_preserves_order_and_item_errors():
    engine = OneOcrEngine.__new__(OneOcrEngine)
    engine.max_inflight = 2

    def operation(value):
        if value == "bad":
            raise ValueError("invalid input")
        return value.upper()

    items = engine._run_batch(["first", "bad", "last"], operation, RecognitionBatchItem)
    assert [item.index for item in items] == [0, 1, 2]
    assert items[0].result == "FIRST"
    assert items[1].result is None and items[1].error.code == "invalid_input"
    assert items[2].result == "LAST"
    assert items[0].to_dict() == {"index": 0, "result": "FIRST"}
    assert items[1].to_dict() == {"index": 1, "error": {"code": "invalid_input", "message": "invalid input"}}


def test_batch_empty_is_empty():
    engine = OneOcrEngine.__new__(OneOcrEngine)
    engine.max_inflight = 1
    assert engine._run_batch([], lambda value: value, RecognitionBatchItem) == []
