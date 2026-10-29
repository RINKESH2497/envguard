import os
import sys
import tempfile
import unittest

# Ensure parent directory is on sys.path for direct execution
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from envguard import Schema, VarSpec, parse_dotenv, validate


class TestEnvGuard(unittest.TestCase):
    def test_parse_dotenv_escapes(self):
        with tempfile.NamedTemporaryFile("w+", delete=False) as f:
            f.write(
                'PORT=8080\nWIN_PATH="C:\\\\new"\nGREETING="hello\\nworld"\nEMPTY=\n'
            )
            f_path = f.name

        try:
            env = parse_dotenv(f_path)
            self.assertEqual(env.get("PORT"), "8080")
            self.assertEqual(env.get("WIN_PATH"), "C:\\new")
            self.assertEqual(env.get("GREETING"), "hello\nworld")
            self.assertEqual(env.get("EMPTY"), "")
        finally:
            if os.path.exists(f_path):
                os.remove(f_path)

    def test_validate_rules(self):
        # Support dict-based Schema initialization
        schema = Schema(
            {
                "version": "1",
                "vars": {
                    "PORT": {"type": "integer", "required": True},
                    "DEBUG": {"type": "boolean", "required": False, "default": "false"},
                    "DATABASE_URL": {
                        "type": "url",
                        "required": True,
                        "pattern": r"^postgres(ql)?://.+",
                    },
                },
            }
        )

        # Invalid env
        bad_result = validate(schema, {"DEBUG": "not_a_bool"})
        self.assertTrue(bad_result.has_errors())

        # Valid env with boolean 'yes'
        good_result = validate(
            schema,
            {
                "PORT": "5432",
                "DEBUG": "yes",
                "DATABASE_URL": "postgresql://user:pass@localhost:5432/db",
            },
        )
        self.assertFalse(good_result.has_errors())
        self.assertEqual(good_result.valid, 3)

        # Also support kwargs and VarSpec objects
        kwarg_schema = Schema(
            version="1",
            vars={
                "PORT": VarSpec(type="integer", required=True),
            },
        )
        self.assertEqual(len(kwarg_schema.vars), 1)


if __name__ == "__main__":
    unittest.main()
