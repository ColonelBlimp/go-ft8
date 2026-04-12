! dump_llr_bin.f90 — Dumps bmet arrays in BINARY float32 format
! for bit-exact comparison with Go pipeline.

program dump_llr_bin

  include '/home/mveary/Development/wsjt-wsjtx/lib/ft8/ft8_params.f90'

  integer MAXCAND
  parameter (MAXCAND=600)
  integer*2 iwave(NMAX)
  real dd(NMAX)
  real candidate(3,MAXCAND)
  real sbase(NH1)
  complex cd0(0:3199)
  complex csymb(32)
  complex cs(0:7,NN)
  real s8(0:7,NN)
  complex ctwk(32)
  real bmeta(174),bmetb(174),bmetc(174),bmetd(174)
  real s2(0:511)
  logical one(0:511,0:8)
  logical newdat
  integer graymap(0:7),icos7(0:6),ihdr(11)
  real ss(9),a(5),scalefac
  real fs2,dt2,twopi,f1,xdt,sync,smax,delf,delfbest,dphi,phi
  real bm,den,cm
  integer i0,ibest,ncand,ios,i,j,k,n3,i3
  integer is1,is2,is3,nsync
  integer ip(1),iloc(1)
  character*256 wavfile

  data icos7/3,1,4,0,6,5,2/
  data graymap/0,1,3,2,5,6,4,7/

  one=.false.
  do i=0,511
    do j=0,8
      if(iand(i,2**j).ne.0) one(i,j)=.true.
    enddo
  enddo
  fs2=12000.0/NDOWN; dt2=1.0/fs2; twopi=8.0*atan(1.0); scalefac=2.83

  call get_command_argument(1,wavfile)
  open(10,file=trim(wavfile),access='stream',status='old',iostat=ios)
  read(10) ihdr; iwave=0; read(10,iostat=ios) iwave; close(10)
  dd=iwave

  call sync8(dd,NMAX,200,2600,1.3,0,MAXCAND,candidate,ncand,sbase)

  ! Process candidate 9
  newdat=.true.
  f1=candidate(1,9); xdt=candidate(2,9)
  write(*,'(A,F10.3,A,F10.4)') 'Candidate 9: freq=',f1,' dt=',xdt

  call ft8_downsample(dd,newdat,f1,cd0)
  i0=nint((xdt+0.5)*fs2); smax=0.0
  do i=i0-10,i0+10
    call sync8d(cd0,i,ctwk,0,sync)
    if(sync.gt.smax) then; smax=sync; ibest=i; endif
  enddo
  smax=0.0; delfbest=0.0
  do i=-5,5
    delf=i*0.5; dphi=twopi*delf*dt2; phi=0.0
    do j=1,32
      ctwk(j)=cmplx(cos(phi),sin(phi)); phi=mod(phi+dphi,twopi)
    enddo
    call sync8d(cd0,ibest,ctwk,1,sync)
    if(sync.gt.smax) then; smax=sync; delfbest=delf; endif
  enddo
  a=0.0; a(1)=-delfbest
  call twkfreq1(cd0,2816,fs2,a,cd0)
  f1=f1+delfbest
  call ft8_downsample(dd,.false.,f1,cd0)
  do i=-4,4
    call sync8d(cd0,ibest+i,ctwk,0,sync); ss(i+5)=sync
  enddo
  iloc=maxloc(ss); ibest=iloc(1)-5+ibest
  xdt=(ibest-1)*dt2
  write(*,'(A,I6,A,F10.3,A,F10.4)') 'Refined: ibest=',ibest,' f1=',f1,' xdt=',xdt

  ! Symbol spectra
  do k=1,NN
    i0=ibest+(k-1)*32; csymb=cmplx(0.0,0.0)
    if(i0.ge.0.and.i0+31.le.2815) csymb=cd0(i0:i0+31)
    call four2a(csymb,32,1,-1,1)
    cs(0:7,k)=csymb(1:8)/1e3; s8(0:7,k)=abs(csymb(1:8))
  enddo

  ! Soft metrics
  bmeta=0; bmetb=0; bmetc=0; bmetd=0
  do nsym=1,3
    nt=2**(3*nsym)
    do ihalf=1,2
      do k=1,29,nsym
        if(ihalf.eq.1) ks=k+7
        if(ihalf.eq.2) ks=k+43
        do i=0,nt-1
          i1=i/64; i2=iand(i,63)/8; i3=iand(i,7)
          if(nsym.eq.1) s2(i)=abs(cs(graymap(i3),ks))
          if(nsym.eq.2) s2(i)=abs(cs(graymap(i2),ks)+cs(graymap(i3),ks+1))
          if(nsym.eq.3) s2(i)=abs(cs(graymap(i1),ks)+cs(graymap(i2),ks+1)+cs(graymap(i3),ks+2))
        enddo
        i32=1+(k-1)*3+(ihalf-1)*87
        if(nsym.eq.1) ibmax=2
        if(nsym.eq.2) ibmax=5
        if(nsym.eq.3) ibmax=8
        do ib=0,ibmax
          bm=maxval(s2(0:nt-1),one(0:nt-1,ibmax-ib)) - &
             maxval(s2(0:nt-1),.not.one(0:nt-1,ibmax-ib))
          if(i32+ib.gt.174) cycle
          if(nsym.eq.1) then
            bmeta(i32+ib)=bm
            den=max(maxval(s2(0:nt-1),one(0:nt-1,ibmax-ib)), &
                    maxval(s2(0:nt-1),.not.one(0:nt-1,ibmax-ib)))
            if(den.gt.0.0) then; cm=bm/den; else; cm=0.0; endif
            bmetd(i32+ib)=cm
          elseif(nsym.eq.2) then
            bmetb(i32+ib)=bm
          elseif(nsym.eq.3) then
            bmetc(i32+ib)=bm
          endif
        enddo
      enddo
    enddo
  enddo
  call normalizebmet(bmeta,174)
  call normalizebmet(bmetb,174)
  call normalizebmet(bmetc,174)
  call normalizebmet(bmetd,174)

  ! Write BINARY bmet arrays (4 x 174 x float32 = 2784 bytes)
  open(20,file='bmet_cand9.bin',access='stream',status='replace')
  write(20) bmeta
  write(20) bmetb
  write(20) bmetc
  write(20) bmetd
  close(20)
  write(*,'(A)') 'Binary bmet written to bmet_cand9.bin'

  ! Also write cd0 for comparison (3200 x complex = 3200 x 2 x float32 = 25600 bytes)
  open(21,file='cd0_cand9.bin',access='stream',status='replace')
  write(21) cd0
  close(21)
  write(*,'(A)') 'Binary cd0 written to cd0_cand9.bin'

end program dump_llr_bin

subroutine normalizebmet(bmet,n)
  real bmet(n)
  bmetav=sum(bmet)/n; bmet2av=sum(bmet*bmet)/n
  var=bmet2av-bmetav*bmetav
  if(var.gt.0.0) then; bmetsig=sqrt(var)
  else; bmetsig=sqrt(bmet2av); endif
  if(bmetsig.gt.0.0) bmet=bmet/bmetsig
  return
end subroutine
