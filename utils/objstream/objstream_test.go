package objstream

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReader_Read_FromEmptyStreamProducesEof(t *testing.T) {
	reader := NewReader[*msg](bytes.NewReader(nil))
	require.ErrorIs(t, reader.Read(nil), io.EOF)
}

func TestReader_Read_CanReadEncodedMessage(t *testing.T) {
	data := []byte{0x2, 0x1, 0x3}
	reader := NewReader[*msg](bytes.NewReader(data))

	var restored msg
	require.NoError(t, reader.Read(&restored))
	require.Equal(t, restored, msg([]byte{0x1, 0x3}))

	require.ErrorIs(t, reader.Read(&restored), io.EOF)
}

func TestReader_Read_ReportsDeserializationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	trg := NewMockDeserializer(ctrl)

	injectedError := fmt.Errorf("injected")
	trg.EXPECT().Deserialize(gomock.Any()).Return(injectedError)

	reader := NewReader[*MockDeserializer](bytes.NewReader([]byte{0x2, 0x1, 0x3}))
	require.ErrorIs(t, reader.Read(trg), injectedError)
}

func TestWriter_Write_EncodesMessageWithLengthPrefix(t *testing.T) {
	buffer := bytes.Buffer{}
	writer := bufio.NewWriter(&buffer)
	out := NewWriter[msg](writer)

	require.NoError(t, out.Write(msg("hello")))
	require.NoError(t, writer.Flush())

	require.Equal(t, []byte{0x5, 'h', 'e', 'l', 'l', 'o'}, buffer.Bytes())
}

func TestWriter_Write_ReportsSerializationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	trg := NewMockSerializer(ctrl)

	injectedError := fmt.Errorf("injected")
	trg.EXPECT().Serialize().Return(nil, injectedError)

	buffer := bytes.Buffer{}
	writer := bufio.NewWriter(&buffer)
	out := NewWriter[*MockSerializer](writer)

	require.ErrorIs(t, out.Write(trg), injectedError)
}

func TestIntegration_CanEncodeAndDecodeListOfMessages(t *testing.T) {
	require := require.New(t)

	input := []string{"hello", "world", "this", "is", "a", "test"}

	buffer := bytes.Buffer{}
	writer := bufio.NewWriter(&buffer)
	out := NewWriter[msg](writer)
	for _, str := range input {
		require.NoError(out.Write(msg(str)))
	}
	require.NoError(writer.Flush())

	in := NewReader[*msg](bufio.NewReader(&buffer))
	var output []string
	for {
		var str msg
		err := in.Read(&str)
		if err == io.EOF {
			break
		}
		require.NoError(err)
		output = append(output, string(str))
	}

	require.Equal(input, output)
}

type msg string

func (m msg) Serialize() ([]byte, error) {
	return []byte(m), nil
}

func (m *msg) Deserialize(data []byte) error {
	*m = msg(data)
	return nil
}
