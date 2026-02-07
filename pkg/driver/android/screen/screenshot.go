package screen

import "trek/pkg/driver"

var _ driver.IScreenshot = (*Screenshot)(nil)

type Screenshot struct {
}

func (s *Screenshot) Screenshot() ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Screenshot) SaveScreenshot(path string) error {
	//TODO implement me
	panic("implement me")
}

func (s *Screenshot) Record(path string) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Screenshot) StopRecording() error {
	//TODO implement me
	panic("implement me")
}

func (s *Screenshot) Close() error {
	//TODO implement me
	panic("implement me")
}
