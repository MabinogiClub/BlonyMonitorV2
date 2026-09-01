import unittest

from update_mabidata_from_pack import item_locale_score, pick_item_row


class ItemSelectionTest(unittest.TestCase):
    def test_target_locale_beats_later_foreign_definition(self) -> None:
        rows = [
            {"Text_Name0": "default"},
            {"Locale": "china", "Text_Name0": "china"},
            {"Locale": "korea", "Text_Name0": "korea"},
        ]

        self.assertEqual(pick_item_row(rows, None, "china")["Text_Name0"], "china")

    def test_default_definition_beats_foreign_collision(self) -> None:
        rows = [
            {"Text_Name0": "Fresh Baristar Hat"},
            {"Locale": "korea", "Text_Name0": "Fresh Baristar Hat"},
        ]

        self.assertIs(pick_item_row(rows, {"unrelatedKoreaFeature"}, "china"), rows[0])

    def test_enabled_feature_beats_base_definition(self) -> None:
        rows = [
            {"Text_Name0": "base"},
            {"feature": "gfChinaUpdate", "Text_Name0": "updated"},
        ]

        self.assertIs(pick_item_row(rows, {"gfChinaUpdate"}, "china"), rows[1])

    def test_later_version_feature_beats_base_without_checklist_override(self) -> None:
        rows = [
            {"Text_Name0": "base"},
            {"feature": "gfOtherRegion", "Text_Name0": "updated"},
        ]

        self.assertIs(pick_item_row(rows, {"gfChinaUpdate"}, "china"), rows[1])

    def test_missing_localization_falls_back_within_same_locale(self) -> None:
        rows = [
            {"Text_Name0": "base", "Text_Name1": "localized"},
            {"feature": "gfUpdate", "Text_Name0": "updated", "Text_Name1": "missing"},
        ]

        self.assertIs(
            pick_item_row(rows, None, "china", lambda row: row["Text_Name1"] == "localized"),
            rows[0],
        )

    def test_exclusion_locale(self) -> None:
        self.assertEqual(item_locale_score("!korea", "china"), 1)
        self.assertIsNone(item_locale_score("!china", "china"))


if __name__ == "__main__":
    unittest.main()
