package util

import "testing"

func TestConvertHTMLToWAStyle(t *testing.T) {
	testCases := []struct {
		input          string
		expectedOutput string
	}{
		{
			"<p><strong>bold </strong><em>italic</em> <strong><em>both</em></strong></p>",
			"*bold *_italic_ *_both_*",
		},
		{
			"<p>test</p><p>test2</p>",
			"test\ntest2",
		},
		{
			"<p>test</p><br><p>test2</p>",
			"test\n\ntest2",
		},
		{
			"<p>geht/das</p>",
			"geht/das",
		},
		{
			"<p><strong><span class=\"ql-emojiblot\" data-name=\"sunglasses\">\ufeff<span contenteditable=\"false\"><span class=\"ap ap-sunglasses\">😎</span></span>\ufeff</span>df</strong></p>",
			"*😎df*",
		},
	}
	for _, testCase := range testCases {
		output := ConvertHTMLToWAStyle(testCase.input)
		if output != testCase.expectedOutput {
			t.Errorf("Want %s, got %s", testCase.expectedOutput, output)
		}
	}
}
