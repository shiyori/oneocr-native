import json

from test_config_cache import write_model

from oneocr_native.cli import main


def test_inspect_is_read_only_and_emits_json(tmp_path, capsys):
    path = tmp_path / "model.onemodel"
    write_model(path)
    before = path.read_bytes()
    assert main(["inspect", "--model", str(path)]) == 0
    result = json.loads(capsys.readouterr().out)
    assert len(result["resources"]) == 2
    assert path.read_bytes() == before
    assert list(tmp_path.iterdir()) == [path]


def test_errors_go_to_stderr_with_nonzero_status(tmp_path, capsys):
    path = tmp_path / "broken.onemodel"
    path.write_bytes(b"invalid")
    assert main(["prepare", "--model", str(path)]) == 2
    captured = capsys.readouterr()
    assert not captured.out and "truncated" in captured.err
