import os
import tempfile
import unittest
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
        schema = Schema(
            version="1",
            vars={
                "PORT": VarSpec(type="integer", required=True),
                "DEBUG": VarSpec(type="boolean", required=False, default="false"),
                "DATABASE_URL": VarSpec(
                    type="url", required=True, pattern=r"^postgres(ql)?://.+"
                ),
            },
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


if __name__ == "__main__":
    unittest.main()
