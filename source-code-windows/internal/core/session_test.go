package core

import (
 "errors"
 "testing"
)
type fakeMemory struct{data map[uint64]byte;alive bool}
func(m *fakeMemory)Info()ProcessInfo{return ProcessInfo{Base:0x10000000,Size:0x1000000}}
func(m *fakeMemory)Read(a uint64,n int)([]byte,error){b:=make([]byte,n);for i:=range b{b[i]=m.data[a+uint64(i)]};return b,nil}
func(m *fakeMemory)Write(a uint64,b []byte)error{if !m.alive{return errors.New("closed")};for i,v:=range b{m.data[a+uint64(i)]=v};return nil}
func(m *fakeMemory)Pause()(func()error,error){return func()error{return nil},nil}
func(m *fakeMemory)Alive()bool{return m.alive}
func(m *fakeMemory)Close()error{m.alive=false;return nil}
func TestPatchRoundTripAndMismatch(t *testing.T){
 p:=Builtins()["geo.invincibility"];m:=&fakeMemory{data:map[uint64]byte{},alive:true}
 for _,g:=range p.Guards{r,_:=Offset(g.RVA);b,_:=Bytes(g.Expected);_ = m.Write(m.Info().Base+r,b)}
 s:=NewSession(m,Image{SHA256:KnownHash,Format:"pe",Arch:"amd64"})
 if e:=s.Enable(p);e!=nil{t.Fatal(e)};r,_:=Offset(p.Patches[0].RVA);b,_:=m.Read(m.Info().Base+r,1);if b[0]!=0xc3{t.Fatal("patch did not apply")}
 if e:=s.Disable(p.ID);e!=nil{t.Fatal(e)};b,_=m.Read(m.Info().Base+r,1);if b[0]!=0x48{t.Fatal("original byte not restored")}
 m.data[m.Info().Base+r]=0x90;if e:=s.Enable(p);e==nil{t.Fatal("mismatch was accepted")};if len(s.Active)!=0{t.Fatal("failed enable retained active state")}
}
func TestBuiltinsAreValid(t *testing.T){mods:=Builtins();if len(mods)!=6{t.Fatalf("expected six recipes, got %d",len(mods))};for _,p:=range mods{if e:=p.Validate();e!=nil{t.Fatal(p.ID,e)}}}
