package pulp

import (
	"fmt"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/magiconair/properties/assert"
)

func TestContentGuardHrefsAreEqual(t *testing.T) {
	var hrefTemplate = "/api/pulp/em%sd/api/v3/contentguards/core/%s/%s/"
	var orgID = faker.UUIDDigit()

	var href1 = fmt.Sprintf(hrefTemplate, orgID, "header", faker.UUIDHyphenated()) // id
	var href2 = fmt.Sprintf(hrefTemplate, orgID, "rbac", faker.UUIDHyphenated())   // rbac
	var href3 = fmt.Sprintf(hrefTemplate, orgID, "header", faker.UUIDHyphenated()) // turnpike
	var href4 = fmt.Sprintf(hrefTemplate, orgID, "header", faker.UUIDHyphenated()) // different

	var baseSlice = []string{href1, href2, href3}
	var sameSizeDiffOrder = []string{href3, href2, href1}
	var sameSizeSameOrder = []string{href1, href2, href3}
	var sameSizeDiffContent = []string{href1, href2, href4}
	var diffSmaller = []string{href1, href2}
	var diffLarger = []string{href1, href2, href3, href4}
	var sameSizeEmptyString = []string{href1, href2, ""}
	var sameSizeEmptyStringDiffOrder = []string{"", href2, href1}
	var sameSizeAllEmptyStrings = []string{"", "", ""}
	var diffSizeSmallerAllEmptyStrings = []string{"", ""}
	var diffSizeLargerAllEmptyStrings = []string{"", "", "", ""}

	t.Run("sameslice", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, baseSlice), true)
	})

	t.Run("samesize_differentorder", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, sameSizeDiffOrder), true)
	})

	t.Run("samesize_sameorder", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, sameSizeSameOrder), true)
	})

	t.Run("negative_samesize_differentcontent", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, sameSizeDiffContent), false)
	})

	t.Run("negative_differentsizesmaller", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, diffSmaller), false)
	})

	t.Run("negative_differentsizelarger", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, diffLarger), false)
	})

	t.Run("empty_both", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual([]string{}, []string{}), true)
	})

	t.Run("negative_empty_a", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual([]string{}, baseSlice), false)
	})

	t.Run("negative_empty_b", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, []string{}), false)
	})

	t.Run("negative_samesize_emptystring", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, sameSizeEmptyString), false)
	})

	t.Run("samesize_oneemptystring_difforder", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(sameSizeEmptyString, sameSizeEmptyStringDiffOrder), true)
	})

	t.Run("negative_samesize_allemptystrings", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(baseSlice, sameSizeAllEmptyStrings), false)
	})

	t.Run("negative_differentsizesmaller_allemptystrings", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(sameSizeAllEmptyStrings, diffSizeSmallerAllEmptyStrings), false)
	})

	t.Run("negative_differentsizelarger_allemptystrings", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(sameSizeAllEmptyStrings, diffSizeLargerAllEmptyStrings), false)
	})

	t.Run("sameslice_allemptystrings", func(t *testing.T) {
		assert.Equal(t, contentGuardHrefsAreEqual(sameSizeAllEmptyStrings, sameSizeAllEmptyStrings), true)
	})
}
