package internal

func (p *Pager) previousFile() {
	newIndex := p.currentReader - 1
	if newIndex < 0 {
		newIndex = 0
	}
	p.currentReader = newIndex
	p.readerSwitched <- struct{}{}
}

func (p *Pager) nextFile() {
	newIndex := p.currentReader + 1
	if newIndex >= len(p.readers) {
		newIndex = len(p.readers) - 1
	}
	p.currentReader = newIndex
	p.readerSwitched <- struct{}{}
}

func (p *Pager) firstFile() {
	p.currentReader = 0
	p.readerSwitched <- struct{}{}
}
